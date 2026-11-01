"""The local app builds the current checkout without fetching exporter source."""
from pathlib import Path
import shutil
import subprocess
import tarfile

ROOT = Path(__file__).resolve().parents[2]
ASSETS = ("Dockerfile", "config.yaml", "run.py", "config.alloy", "DOCS.md", ".dockerignore", ".gitignore")


def package(root, destination):
    subprocess.run(["sh", str(root / "scripts/package-homeassistant.sh"), str(destination)], check=True)
    with tarfile.open(destination / "exporter-source.tar") as archive:
        return {member.name.removeprefix("./"): archive.extractfile(member).read()
                for member in archive.getmembers() if member.isfile()}


def test_package_contains_current_build_inputs_only(tmp_path):
    destination = tmp_path / "app with spaces"
    files = package(ROOT, destination)
    sources = [ROOT / "LICENSE", ROOT / "go.mod", ROOT / "go.sum", *ROOT.glob("*.go"),
               *(ROOT / "config").glob("*.go"), *(ROOT / "myuplink").glob("*.go")]
    assert files == {str(path.relative_to(ROOT)): path.read_bytes() for path in sources}
    for asset in ASSETS:
        assert (destination / asset).read_bytes() == (ROOT / "homeassistant/myuplink_cloud" / asset).read_bytes()


def test_repackage_updates_changed_sources_and_removes_deleted_sources(tmp_path):
    root, destination = tmp_path / "checkout", tmp_path / "app"
    (root / "scripts").mkdir(parents=True)
    shutil.copy(ROOT / "scripts/package-homeassistant.sh", root / "scripts")
    app = root / "homeassistant/myuplink_cloud"
    app.mkdir(parents=True)
    for asset in ASSETS:
        shutil.copy(ROOT / "homeassistant/myuplink_cloud" / asset, app)
    for name in ("LICENSE", "go.mod", "go.sum", "main.go", "obsolete.go", "config/opts.go", "myuplink/client.go"):
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("original")
    assert "obsolete.go" in package(root, destination)
    (root / "main.go").write_text("current uncommitted source")
    (root / "obsolete.go").unlink()
    (root / "myuplink.log").write_text("private log data")
    (root / ".env").write_text("private credentials")
    files = package(root, destination)
    assert files["main.go"] == b"current uncommitted source"
    assert "obsolete.go" not in files
    assert "myuplink.log" not in files
    assert ".env" not in files
