# Pastel for macOS 15 (unofficial fork)

This fork adapts [Pastel](https://github.com/EEliberto/Pastel-macOS) to Apple Silicon Macs running macOS 15. It replaces macOS 26 interface APIs with older controls and disables the upstream auto-updater. It also includes an optional preparation tool that retrieves and integrity-checks the required Apple system files before signing in, with progress for the final kernel component. It does not bundle those Apple files or account data.

**Status:** The app was built and launched on macOS 15.8.1; app search, version history and preparation of the Apple files were verified. Real Apple account sign-in and an end-to-end IPA download have not yet been verified on this fork.

## Build

Requirements: Apple Silicon Mac, macOS 15 or later, Apple command line developer tools with a macOS 15 SDK, Go 1.26.5 or later, Python 3, network access. The script downloads Sparkle 2.10.1 from its official release and checks its SHA-256. It installs the locked Node dependencies with the project's bundled Node runtime.

```sh
sh Scripts/BuildMacOS15.sh
```

The script creates `dist/Pastel 15.app` and `dist/pastel-prepare`; use `PASTEL_OUTPUT_DIR` to choose a different directory. It refuses to overwrite an existing app. The app is ad-hoc signed, not notarized; macOS may warn on opening. The CLI build does not include the original application icon and supports the simple string translations in the source catalog.

## Prepare before signing in

```sh
./dist/pastel-prepare
```

The tool downloads selected components directly from a pinned Apple restore image, checks their expected sizes and SHA-256 hashes, and stores them in the local ipatool cache. It prints the amount transferred and progress for the final kernel archive; the former is not a percentage. Allow up to 30 minutes for a slow network. It does not ask for an Apple Account or password. A successful result means the Apple files are cached; other authentication dependencies and the actual login may still need network access.

After preparation, open the app and add your Apple Account. Free apps may need to be acquired on the account; paid apps must already be purchased. Actual availability depends on Apple and the selected version. Keep credentials and 2FA codes private.

## Maintenance

The Apple restore image URL, pinned file hashes, and authentication implementation are inherited from the vendored ipatool snapshot. Apple can change its authentication or delivery services, so a populated cache cannot guarantee future compatibility. Updates may require a coordinated upstream refresh. Do not publish Apple binaries, account sessions, downloaded IPAs or credentials in this repository.

The original project is [EEliberto/Pastel-macOS](https://github.com/EEliberto/Pastel-macOS), licensed under Apache-2.0. Third-party licenses remain in their respective directories.
