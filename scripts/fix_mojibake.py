#!/usr/bin/env python3
"""Fix HTML/JS/CSS mangled by PowerShell Set-Content (cp1252 / U+FFFD)."""
from __future__ import annotations

import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parents[1] / "web"
BAD = "\ufffd"


def read_text(path: pathlib.Path) -> str:
    raw = path.read_bytes()
    for enc in ("utf-8", "utf-8-sig", "cp1252", "latin-1"):
        try:
            return raw.decode(enc)
        except UnicodeDecodeError:
            continue
    return raw.decode("utf-8", errors="replace")


def fix_text(t: str) -> str:
    # Already-decoded Windows specials that often appear after bad saves
    t = t.replace("\u0097", "—")  # C1 control sometimes left from cp1252 mishandling
    # 1024?768 → 1024x768
    t = re.sub(rf"(\d){BAD}(\d)", r"\1x\2", t)
    # word ? word → em dash
    t = re.sub(rf"(\w)\s*{BAD}\s*(\w)", r"\1 — \2", t)
    # before closing quote/tag → ellipsis
    t = re.sub(rf"{BAD}(?=[\"'<])", "…", t)
    t = re.sub(rf"{BAD}(?=</)", "…", t)
    # leftover replacement → em dash
    t = t.replace(BAD, "—")
    return t


def main() -> None:
    fixed = 0
    for p in sorted(
        list(ROOT.rglob("*.html")) + list(ROOT.rglob("*.js")) + list(ROOT.rglob("*.css"))
    ):
        text = read_text(p)
        out = fix_text(text)
        # Always rewrite as UTF-8 if encoding was wrong or content changed
        raw = p.read_bytes()
        try:
            raw.decode("utf-8")
            utf8_ok = True
        except UnicodeDecodeError:
            utf8_ok = False
        if out != text or not utf8_ok:
            p.write_text(out, encoding="utf-8", newline="\n")
            fixed += 1
            print("fixed", p.relative_to(ROOT), "(utf8)" if utf8_ok else "(reencoded)")
    print("total", fixed)

    left = []
    for p in ROOT.rglob("*"):
        if p.suffix not in {".html", ".js", ".css"}:
            continue
        try:
            if BAD in p.read_text(encoding="utf-8"):
                left.append(str(p.relative_to(ROOT)))
        except UnicodeDecodeError:
            left.append(str(p.relative_to(ROOT)) + " (still not utf-8)")
    print("remaining", left or "none")


if __name__ == "__main__":
    main()
