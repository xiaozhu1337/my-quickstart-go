"""Build native launchers with the Go toolchain managed by mise."""
import argparse
import os
from pathlib import Path
import plistlib
import platform
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def build(target_os: str, arch: str) -> Path:
    output = ROOT / "bin" / f"{target_os}-{arch}"
    if target_os == "darwin":
        bundle = output / "My Quickstart Go.app"
        binary = bundle / "Contents" / "MacOS" / "my-quickstart-go"
    else:
        binary = output / ("my-quickstart-go.exe" if target_os == "windows" else "my-quickstart-go")
    binary.parent.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env.update(GOOS=target_os, GOARCH=arch, CGO_ENABLED="0")
    flags = "-s -w" + (" -H=windowsgui" if target_os == "windows" else "")
    subprocess.run(["mise", "exec", "go@1.27.1", "--", "go", "build", "-trimpath", "-ldflags", flags,
                    "-o", str(binary), "./cmd/quickstart"], cwd=ROOT, env=env, check=True, stdin=subprocess.DEVNULL)
    if target_os == "darwin":
        info = {"CFBundleName": "My Quickstart Go", "CFBundleDisplayName": "My Quickstart Go",
                "CFBundleIdentifier": "com.cat.my-quickstart-go", "CFBundleExecutable": binary.name,
                "CFBundlePackageType": "APPL", "CFBundleVersion": "1", "CFBundleShortVersionString": "1.0.0",
                "CFBundleIconFile": "icon.icns", "NSHighResolutionCapable": True}
        resources = bundle / "Contents" / "Resources"
        resources.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / "internal" / "branding" / "icon.icns", resources / "icon.icns")
        with (bundle / "Contents" / "Info.plist").open("wb") as handle:
            plistlib.dump(info, handle)
    return binary


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--os", choices=["windows", "linux", "darwin"],
                        default={"win32": "windows", "darwin": "darwin"}.get(sys.platform, "linux"))
    parser.add_argument("--arch", choices=["amd64", "arm64"],
                        default="arm64" if platform.machine().lower() in {"aarch64", "arm64"} else "amd64")
    args = parser.parse_args()
    print(build(args.os, args.arch))
