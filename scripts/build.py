"""Build only windows-amd64 with the Go toolchain managed by mise."""
import argparse
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def build(target_os: str = "windows", arch: str = "amd64") -> Path:
    if (target_os, arch) != ("windows", "amd64"):
        raise ValueError("Only windows-amd64 builds are supported")
    binary = ROOT / "bin" / "windows-amd64" / "my-quickstart-go.exe"
    binary.parent.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env.update(GOOS=target_os, GOARCH=arch, CGO_ENABLED="0")
    flags = "-s -w -H=windowsgui"
    subprocess.run(["mise", "exec", "go@1.27.3", "--", "go", "build", "-trimpath", "-ldflags", flags,
                    "-o", str(binary), "./cmd/quickstart"], cwd=ROOT, env=env, check=True, stdin=subprocess.DEVNULL,
                   creationflags=subprocess.CREATE_NO_WINDOW if sys.platform == "win32" else 0)
    return binary


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--os", choices=["windows"], default="windows")
    parser.add_argument("--arch", choices=["amd64"], default="amd64")
    args = parser.parse_args()
    print(build(args.os, args.arch))
