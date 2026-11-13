//nolint:wsl // Archive extraction is clearer when resource setup remains adjacent.
package assets

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	appleMediaServicesImage = "/System/Library/PrivateFrameworks/AppleMediaServices.framework/Versions/A/AppleMediaServices"
	fileSystemImageName     = "043-70867-640.dmg.aea"
	systemOSImageName       = "043-70701-651.dmg.aea"
	fileSystemArchiveSize   = 11274289152
	fileSystemImageSize     = 14189330432
	systemOSArchiveSize     = 2415919104
	systemOSImageSize       = 2879389696
	arm64CachePath          = "/System/Library/dyld/dyld_shared_cache_arm64e"
)

var frameworkImagePaths = []string{
	"/System/Library/PrivateFrameworks/CoreADI.framework/Versions/A/adid",
	"/System/Library/PrivateFrameworks/CommerceKit.framework/Versions/A/Resources/commerce",
	"/System/Library/PrivateFrameworks/CoreFP.framework/Versions/A/CoreFP",
	"/System/Library/PrivateFrameworks/CoreFP.framework/Versions/A/Resources/CoreFP.icxs",
	"/System/Library/PrivateFrameworks/CoreFP.framework/Versions/A/fairplayd",
}

func resolveRemoteImage(ctx context.Context) (Bundle, error) {
	assetProgress("connecting", 0, 0)
	client := &http.Client{}
	remote, err := openRemoteFile(ctx, client)
	if err != nil {
		return Bundle{}, fmt.Errorf("resolve macOS %s (%s) frameworks: %w", macOSVersion, macOSBuild, err)
	}

	archive, err := zip.NewReader(remote, remote.size)
	if err != nil {
		return Bundle{}, fmt.Errorf("read macOS %s (%s) restore image: %w", macOSVersion, macOSBuild, err)
	}

	fileSystemEntry, err := pinnedZipEntry(archive, fileSystemImageName, fileSystemArchiveSize, fileSystemArchiveSize)
	if err != nil {
		return Bundle{}, err
	}

	systemOSEntry, err := pinnedZipEntry(archive, systemOSImageName, systemOSArchiveSize, systemOSArchiveSize)
	if err != nil {
		return Bundle{}, err
	}

	workspace, err := os.MkdirTemp("", "ipatool-macos-27-*")
	if err != nil {
		return Bundle{}, fmt.Errorf("create macOS image workspace: %w", err)
	}
	defer os.RemoveAll(workspace)

	frameworkDirectory := filepath.Join(workspace, "frameworks")
	reused := reuseFrameworks(frameworkDirectory) == nil
	if !reused {
		fmt.Fprintln(os.Stderr, "正在提取 Apple 系统文件……")
		if err := extractFrameworkFiles(ctx, client, remote, fileSystemEntry, frameworkDirectory); err != nil {
			return Bundle{}, err
		}

	}
	assetProgress("AppleMediaServices", 0, 0)
	fmt.Fprintln(os.Stderr, "正在读取 AppleMediaServices 系统框架……")
	appleMediaServicesPath := filepath.Join(frameworkDirectory, "AppleMediaServices")
	if !reused {
		if err := extractAppleMediaServices(ctx, client, remote, systemOSEntry, appleMediaServicesPath); err != nil {
			return Bundle{}, fmt.Errorf("extract AppleMediaServices from macOS %s (%s): %w", macOSVersion, macOSBuild, err)
		}
	}
	fmt.Fprintln(os.Stderr, "正在分块下载最后的内核组件（约 31.5 MB）……")
	kernel, err := pinnedZipEntry(archive, kernelCacheName, kernelCacheCompressedSize, kernelCacheArchiveSize)
	if err != nil {
		return Bundle{}, err
	}
	start, err := kernel.DataOffset()
	if err != nil {
		return Bundle{}, err
	}
	buf := make([]byte, kernelCacheCompressedSize)
	assetProgress("FairPlayIOKit", 0, int64(len(buf)))
	for pos := 0; pos < len(buf); {
		end := pos + 1024*1024
		if end > len(buf) {
			end = len(buf)
		}
		n, err := remote.ReadAt(buf[pos:end], start+int64(pos))
		if err != nil {
			return Bundle{}, err
		}
		if n != end-pos {
			return Bundle{}, io.ErrUnexpectedEOF
		}
		pos = end
		assetProgress("FairPlayIOKit", int64(pos), int64(len(buf)))
		fmt.Fprintf(os.Stderr, "内核组件: %.1f / %.1f MB\n", float64(pos)/1048576, float64(len(buf))/1048576)
	}
	remote.cacheStart, remote.cacheData = start, buf
	if err := extractFairPlayKext(archive, filepath.Join(frameworkDirectory, "FairPlayIOKit")); err != nil {
		return Bundle{}, err
	}

	assetProgress("verifying", 0, 0)
	files := make(map[string][]byte, len(requiredFiles))

	for _, spec := range requiredFiles {
		data, err := os.ReadFile(filepath.Join(frameworkDirectory, spec.name))
		if err != nil {
			return Bundle{}, fmt.Errorf("read extracted %s: %w", spec.name, err)
		}

		files[spec.name] = data
	}

	bundle := bundleFrom(files)
	if err := validate(bundle); err != nil {
		return Bundle{}, err
	}

	return bundle, nil
}

func extractAppleMediaServices(ctx context.Context, client *http.Client, remote *remoteFile, entry *zip.File, destination string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("extract AppleMediaServices: %w", err)
	}

	offset, err := entry.DataOffset()
	if err != nil {
		return fmt.Errorf("locate encrypted SystemOS cryptex: %w", err)
	}
	encryptedImage := io.NewSectionReader(remote, offset, int64(entry.CompressedSize64))
	image, err := newAEAReader(ctx, client, encryptedImage, int64(entry.CompressedSize64))
	if err != nil {
		return fmt.Errorf("open encrypted SystemOS cryptex: %w", err)
	}

	if image.size != systemOSImageSize {
		return fmt.Errorf("SystemOS cryptex has size %d, expected %d", image.size, systemOSImageSize)
	}

	extractor, closeExtractor, err := newAPFSExtractor(image, image.size)
	if err != nil {
		return err
	}

	defer func() { _ = closeExtractor() }()

	opener := func(suffix string) (*dyldCacheFile, error) {
		file, err := extractor.openFile(ctx, arm64CachePath+suffix)
		if err != nil {
			return nil, fmt.Errorf("open arm64e shared-cache member %q: %w", suffix, err)
		}

		return &dyldCacheFile{reader: file, size: file.size, close: func() error { return nil }}, nil
	}
	if err := extractDyldImageFromFiles(opener, appleMediaServicesImage, destination); err != nil {
		return fmt.Errorf("export AppleMediaServices from arm64e shared cache: %w", err)
	}

	return nil
}

func pinnedZipEntry(archive *zip.Reader, name string, compressedSize, uncompressedSize uint64) (*zip.File, error) {
	expectedMethod := zip.Store
	if compressedSize != uncompressedSize {
		expectedMethod = zip.Deflate
	}

	for _, entry := range archive.File {
		if entry.Name != name {
			continue
		}

		if entry.Method != expectedMethod || entry.CompressedSize64 != compressedSize || entry.UncompressedSize64 != uncompressedSize {
			return nil, fmt.Errorf("macOS %s (%s) restore member %s does not match the pinned image", macOSVersion, macOSBuild, name)
		}

		return entry, nil
	}

	return nil, fmt.Errorf("macOS %s (%s) restore image does not contain %s", macOSVersion, macOSBuild, name)
}

func extractFrameworkFiles(ctx context.Context, client *http.Client, remote *remoteFile, entry *zip.File, destination string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("extract framework files: %w", err)
	}

	offset, err := entry.DataOffset()
	if err != nil {
		return fmt.Errorf("locate encrypted macOS filesystem: %w", err)
	}

	encryptedImage := io.NewSectionReader(remote, offset, int64(entry.CompressedSize64))
	image, err := newAEAReader(ctx, client, encryptedImage, int64(entry.CompressedSize64))
	if err != nil {
		return fmt.Errorf("open encrypted macOS filesystem: %w", err)
	}

	if image.size != fileSystemImageSize {
		return fmt.Errorf("macOS filesystem has size %d, expected %d", image.size, fileSystemImageSize)
	}

	if err := extractAPFSFiles(ctx, image, image.size, frameworkImagePaths, destination); err != nil {
		return fmt.Errorf("extract frameworks from macOS %s (%s): %w", macOSVersion, macOSBuild, err)
	}

	return nil
}

// reuseFrameworks accepts only previously extracted files that match upstream hashes.
func reuseFrameworks(destination string) error {
	source := os.Getenv("PASTEL_PREPARED_DIR")
	if source == "" {
		return fmt.Errorf("没有可复用的文件")
	}
	names := []string{"adid", "commerce", "CoreFP", "CoreFP.icxs", "fairplayd", "AppleMediaServices"}
	if err := os.MkdirAll(destination, 0700); err != nil {
		return err
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			return err
		}
		valid := false
		for _, spec := range requiredFiles {
			if spec.name == name && len(data) == spec.size && sha256.Sum256(data) == spec.digest {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("%s 校验失败", name)
		}
		if err := os.WriteFile(filepath.Join(destination, name), data, 0600); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "已校验并复用:", name)
	}
	return nil
}
