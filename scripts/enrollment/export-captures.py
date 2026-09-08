"""Export selected Phase 7 messages, preserving XML layout and redacting identities.

Usage: python scripts/enrollment/export-captures.py CAPTURE_DIR OUTPUT_DIR
Raw SOAP, passwords, certificates and attestation blobs are never exported.
Review the resulting XML before committing it. Uses only Python's standard library.
"""
import argparse
import json
import re
from pathlib import Path
import xml.etree.ElementTree as ET

NS = {"s": "SYNCML:SYNCML1.2"}
FIXTURE_ID = "0123456789ABCDEF0123456789ABCDEF"


def scrub(text):
    # Keep native element order, whitespace and namespace declarations intact.
    text = re.sub(r"[A-Fa-f0-9]{32}", FIXTURE_ID, text)
    text = text.replace("https://localhost:8443", "https://mdm.example.test")
    for leaf, value in (("Man", "Example Manufacturer"), ("Mod", "Conformance Device")):
        pattern = r"(<LocURI>\./DevInfo/" + leaf + r"</LocURI></Source><Data>)[^<]*(</Data>)"
        text = re.sub(pattern, lambda m: m[1] + value + m[2], text)
    text = re.sub(r"(<LocName>)[^<]*(</LocName>)", r"\g<1>fixture-client\g<2>", text)
    for tag in ("Cred", "Chal"):
        def redact_block(match):
            block = re.sub(r"(<Data>)[^<]*(</Data>)", r"\g<1>AAAAAAAAAAAAAAAAAAAAAA==\g<2>", match[0])
            return re.sub(r"(<NextNonce(?: [^>]*)?>)[^<]*(</NextNonce>)", r"\g<1>AAAAAAAAAAAAAAAAAAAAAA==\g<2>", block)
        text = re.sub(r"<" + tag + r">.*?</" + tag + r">", redact_block, text, flags=re.S)
    # These selected probes need no account names, SIDs, MACs or certificates.
    if re.search(r"Password|BinarySecurityToken|S-1-5-21-|(?:[0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}", text):
        raise ValueError("unexpected identifying data in selected SyncML capture")
    ET.fromstring(text)
    return text


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("capture_dir", type=Path)
    parser.add_argument("output_dir", type=Path)
    args = parser.parse_args()
    chosen = {}
    context = []
    build = None
    for path in sorted(args.capture_dir.glob("*-metadata.json")):
        meta = json.loads(path.read_text())
        prefix = str(path).removesuffix("metadata.json")
        body = Path(prefix + "request.xml").read_text(encoding="utf-8-sig")
        if not body.strip():
            continue
        root = ET.fromstring(body)
        label = meta["experiment"]["label"]
        if meta["path"].endswith("/Enrollment.svc"):
            items = []
            for item in root.iter():
                if item.tag.split("}")[-1] != "ContextItem":
                    continue
                name, value = item.attrib["Name"], "".join(item.itertext())
                entry = {"name": name, "characters": len(value)}
                if name in ("RequestVersion", "EnrollmentType", "OSVersion", "ApplicationVersion", "AttestationStatus", "AttestationStatusHResult"):
                    entry["value"] = value
                if name == "OSVersion":
                    build = value.removeprefix("10.0.")
                items.append(entry)
            context.append({"label": label, "at": meta["at"], "advertised_version": meta["experiment"].get("enrollment_version", "3.0"), "items": items})
            continue
        if not meta["path"].endswith("/MDM.svc"):
            continue
        session = root.findtext("s:SyncHdr/s:SessionID", namespaces=NS)
        msg = root.findtext("s:SyncHdr/s:MsgID", namespaces=NS)
        codes = [a.findtext("s:Data", namespaces=NS) for a in root.findall("s:SyncBody/s:Alert", NS)]
        names = []
        if label == "baseline" and session == "1":
            names = {"1": ["package1-user"], "2": ["package1-digest"], "3": ["devdetail-results"]}.get(msg, [])
        if label == "baseline" and session == "2" and msg == "3":
            names = ["capability-probes"]
        if label == "namespace11" and msg == "3":
            names = ["namespace11-results"]
        if label.startswith("chunked-upload") and "1223" in codes:
            names = ["large-result-abort-" + str(meta["experiment"]["max_msg_size"])]
        if "1226" in codes:
            names = ["unenroll-1226"]
        for name in names:
            chosen.setdefault(name, (scrub(body), meta))
        if label == "namespace11" and msg == "2":
            chosen.setdefault("namespace11-response", (scrub(Path(prefix + "response.xml").read_text()), meta))
    if build is None:
        raise ValueError("no native enrollment OSVersion found")
    args.output_dir.mkdir(parents=True, exist_ok=True)
    manifest = {"build": build, "source": "native Windows HTTP bodies captured by the localhost harness", "fixtures": []}
    for name, (body, meta) in chosen.items():
        filename = f"windows-{build}-{name}.xml"
        (args.output_dir / filename).write_text(body, encoding="utf-8")
        manifest["fixtures"].append({"file": filename, "at": meta["at"], "experiment": meta["experiment"]})
    (args.output_dir / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    (args.output_dir / "enrollment-context.json").write_text(json.dumps(context, indent=2) + "\n", encoding="utf-8")
    print(f"Exported {len(chosen)} redacted SyncML fixtures and {len(context)} enrollment summaries for {build}.")


if __name__ == "__main__":
    main()
