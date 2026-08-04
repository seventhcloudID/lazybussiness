from pathlib import Path

p = Path(r"G:\threads\web\index.html")
t = p.read_text(encoding="utf-8")
t2 = (
    t.replace('"JetBrains Mono"', '"Sora"')
    .replace('"Plus Jakarta Sans"', '"Sora"')
    .replace('"Inter"', '"Sora"')
)
p.write_text(t2, encoding="utf-8", newline="\n")
print("done", t.count("JetBrains"), "->", t2.count("JetBrains"))
