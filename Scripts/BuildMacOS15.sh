#!/bin/bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
out="${PASTEL_OUTPUT_DIR:-$root/dist}"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/pastel15-build.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
app="$out/Pastel 15.app"
[ "$(uname -m)" = arm64 ] || { echo 'Apple Silicon is required.' >&2; exit 1; }
command -v swiftc >/dev/null
command -v go >/dev/null
mkdir -p "$out" "$tmp/framework"
curl -fL --retry 3 -o "$tmp/sparkle.tar.xz" https://github.com/sparkle-project/Sparkle/releases/download/2.10.1/Sparkle-2.10.1.tar.xz
printf '%s  %s\n' 1a856d2a9edb8e628de88121d7be04cf0e2167f8641650b78fca00b14244cf27 "$tmp/sparkle.tar.xz" | shasum -a 256 -c -
tar -xf "$tmp/sparkle.tar.xz" -C "$tmp/framework" ./Sparkle.framework
mkdir -p "$tmp/Pastel 15.app/Contents/MacOS" "$tmp/Pastel 15.app/Contents/Frameworks" "$tmp/Pastel 15.app/Contents/Resources/node/bin"
build="$tmp/Pastel 15.app/Contents"
ditto "$tmp/framework/Sparkle.framework" "$build/Frameworks/Sparkle.framework"
swiftc -target arm64-apple-macosx15.0 -F "$tmp/framework" -framework Sparkle -Xlinker -rpath -Xlinker @executable_path/../Frameworks "$root"/Pastel/*.swift -o "$build/MacOS/Pastel"
mkdir -p "$tmp/NodeProject"
rsync -a --exclude node_modules "$root/NodeProject/" "$tmp/NodeProject/"
gzip -dc "$root/node/bin/node.gz" > "$build/Resources/node/bin/node"
chmod +x "$build/Resources/node/bin/node"
(cd "$tmp/NodeProject" && "$build/Resources/node/bin/node" "$root/node/lib/node_modules/npm/bin/npm-cli.js" ci --omit=dev)
rm -rf "$tmp/NodeProject/test"
ditto "$tmp/NodeProject" "$build/Resources/NodeProject"
gzip -dc "$root/node/bin/store-helper.gz" > "$build/Resources/store-helper"
chmod +x "$build/Resources/store-helper"
xcrun clang -fobjc-arc -fblocks -arch arm64 -mmacosx-version-min=15.0 -framework Foundation "$root/Scripts/SAPSigner.m" -o "$build/Resources/sap-signer"
cp "$root/Pastel/PastelAboutIcon.png" "$build/Resources/"
cp "$root/StoreHelper/ipatool/LICENSE" "$build/Resources/ipatool-LICENSE.txt"
python3 - "$root" "$build" <<'PY'
import json, plistlib, sys
from pathlib import Path
root, build = map(Path,sys.argv[1:]); res=build/'Resources'
p=plistlib.loads((root/'Pastel/Info.plist').read_bytes())
p.update(CFBundleExecutable='Pastel', CFBundleIdentifier='com.allenmiao.ipahistorydownload.macos15', CFBundleName='Pastel 15', CFBundleShortVersionString='20261110', CFBundleVersion='2026111002', LSMinimumSystemVersion='15.0', SUEnableAutomaticChecks=False)
p.pop('SUFeedURL',None); p.pop('CFBundleIconName',None)
(build/'Info.plist').write_bytes(plistlib.dumps(p))
langs={}
for key,item in json.loads((root/'Pastel/Localizable.xcstrings').read_text())['strings'].items():
 for lang,loc in item.get('localizations',{}).items():
  value=loc.get('stringUnit',{}).get('value')
  if value is not None: langs.setdefault(lang,{})[key]=value
for lang,strings in langs.items():
 folder=res/(lang+'.lproj'); folder.mkdir(exist_ok=True)
 (folder/'Localizable.strings').write_bytes(plistlib.dumps(strings,fmt=plistlib.FMT_BINARY))
PY
codesign --force --sign - "$build/Resources/sap-signer"
codesign --force --sign - --options runtime --entitlements "$root/Scripts/NodeJIT.entitlements" "$build/Resources/node/bin/node"
codesign --force --sign - --options runtime --entitlements "$root/Scripts/StoreHelper.entitlements" "$build/Resources/store-helper"
codesign --force --sign - "$build/Frameworks/Sparkle.framework"
codesign --force --sign - "$tmp/Pastel 15.app"
codesign --verify --deep --strict "$tmp/Pastel 15.app"
[ ! -e "$app" ] || { echo "Output already exists: $app. Set PASTEL_OUTPUT_DIR to a new directory." >&2; exit 1; }
ditto "$tmp/Pastel 15.app" "$app"
(cd "$root/StoreHelper" && go build -o "$out/pastel-prepare" ./cmd/pastel-prepare)
echo "Built $app and $out/pastel-prepare"
