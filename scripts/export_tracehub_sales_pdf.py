# -*- coding: utf-8 -*-
"""Markdown → PDF（微软雅黑）。默认：docs/TraceHub-销售介绍.md。

用法: py -3 scripts/export_tracehub_sales_pdf.py [输入.md] [输出.pdf]
依赖: pip install reportlab
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import cm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle

FONT_PATH = r"C:\Windows\Fonts\msyh.ttc"
FONT_NAME = "YaHei"


def register_font() -> None:
    if not Path(FONT_PATH).is_file():
        print(
            f"Font not found: {FONT_PATH}\n"
            "Edit FONT_PATH in this script to a local .ttf/.ttc (e.g. SimSun).",
            file=sys.stderr,
        )
        sys.exit(1)
    pdfmetrics.registerFont(TTFont(FONT_NAME, FONT_PATH, subfontIndex=0))


def md_inline_to_markup(text: str) -> str:
    text = text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    text = re.sub(r"\*\*(.+?)\*\*", r"<b>\1</b>", text)
    text = re.sub(r"`([^`]+)`", r'<font name="Courier">\1</font>', text)
    return text


def is_table_row(line: str) -> bool:
    s = line.strip()
    return s.startswith("|") and s.endswith("|") and s.count("|") >= 2


def parse_table_row(line: str) -> list[str]:
    return [c.strip() for c in line.strip().strip("|").split("|")]


def is_separator_row(cells: list[str]) -> bool:
    if not cells:
        return False
    return all(re.match(r"^:?-+:?$", c.replace(" ", "")) for c in cells if c)


def build_styles():
    register_font()
    base = getSampleStyleSheet()

    def ps(name: str, parent: str, font_size: int, space_before: float = 0, space_after: float = 6, bold: bool = False):
        p = ParagraphStyle(
            name,
            parent=base[parent],
            fontName=FONT_NAME,
            fontSize=font_size,
            leading=font_size * 1.25,
            alignment=TA_LEFT,
            spaceBefore=space_before,
            spaceAfter=space_after,
        )
        if bold:
            # 黑体用粗体字号略增模拟
            p.fontName = FONT_NAME
        return p

    return {
        "title": ps("TH_Title", "Title", 18, space_before=0, space_after=12, bold=True),
        "h1": ps("TH_H1", "Heading1", 14, space_before=14, space_after=8, bold=True),
        "h2": ps("TH_H2", "Heading2", 12, space_before=10, space_after=6, bold=True),
        "body": ps("TH_Body", "Normal", 11, space_before=0, space_after=6),
        "bullet": ps("TH_Bullet", "Normal", 11, space_before=0, space_after=4),
        "italic": ps("TH_Italic", "Normal", 10, space_before=6, space_after=6),
    }


def main() -> None:
    root = Path(__file__).resolve().parents[1]
    # 用法：py -3 scripts/export_tracehub_sales_pdf.py [输入.md] [输出.pdf]
    # 省略参数时默认生成「销售介绍」PDF。
    if len(sys.argv) >= 3:
        md_arg, pdf_arg = sys.argv[1], sys.argv[2]
        md_path = Path(md_arg) if Path(md_arg).is_absolute() else (root / md_arg)
        out_path = Path(pdf_arg) if Path(pdf_arg).is_absolute() else (root / pdf_arg)
    elif len(sys.argv) == 1:
        md_path = root / "docs" / "TraceHub-销售介绍.md"
        out_path = root / "docs" / "TraceHub-销售介绍.pdf"
    else:
        print(
            "用法: py -3 scripts/export_tracehub_sales_pdf.py [输入.md] [输出.pdf]\n"
            "示例: py -3 scripts/export_tracehub_sales_pdf.py docs/TraceHub-客户接入宣传.md docs/TraceHub-客户接入宣传.pdf",
            file=sys.stderr,
        )
        sys.exit(2)

    if not md_path.is_file():
        print(f"Not found: {md_path}", file=sys.stderr)
        sys.exit(1)

    styles = build_styles()
    lines = md_path.read_text(encoding="utf-8").splitlines()

    story: list = []
    doc = SimpleDocTemplate(
        str(out_path),
        pagesize=A4,
        leftMargin=2 * cm,
        rightMargin=2 * cm,
        topMargin=2 * cm,
        bottomMargin=2 * cm,
    )

    i = 0
    while i < len(lines):
        line = lines[i]
        raw = line.rstrip()

        if raw.strip() == "":
            i += 1
            continue

        if raw.strip() == "---":
            story.append(Spacer(1, 6))
            i += 1
            continue

        if is_table_row(raw):
            table_lines: list[str] = []
            while i < len(lines) and is_table_row(lines[i].rstrip()):
                table_lines.append(lines[i].rstrip())
                i += 1
            rows: list[list[str]] = []
            for tl in table_lines:
                cells = parse_table_row(tl)
                if is_separator_row(cells):
                    continue
                rows.append([md_inline_to_markup(c) for c in cells])
            if rows:
                ncol = max(len(r) for r in rows)
                data = []
                for r in rows:
                    row = r + [""] * (ncol - len(r))
                    data.append([Paragraph(c, styles["body"]) for c in row])
                tw = doc.width
                col_w = tw / ncol
                t = Table(data, colWidths=[col_w] * ncol)
                t.setStyle(
                    TableStyle(
                        [
                            ("GRID", (0, 0), (-1, -1), 0.5, colors.grey),
                            ("VALIGN", (0, 0), (-1, -1), "TOP"),
                            ("BACKGROUND", (0, 0), (-1, 0), colors.whitesmoke),
                        ]
                    )
                )
                story.append(Spacer(1, 6))
                story.append(t)
                story.append(Spacer(1, 8))
            continue

        m = re.match(r"^(#{1,3})\s+(.*)$", raw)
        if m:
            n = len(m.group(1))
            title = md_inline_to_markup(m.group(2).strip())
            if n == 1:
                story.append(Paragraph(title, styles["title"]))
            elif n == 2:
                story.append(Paragraph(title, styles["h1"]))
            else:
                story.append(Paragraph(title, styles["h2"]))
            i += 1
            continue

        if raw.lstrip().startswith(("- ", "* ")):
            text = md_inline_to_markup(raw.lstrip()[2:].strip())
            story.append(Paragraph(f"• {text}", styles["bullet"]))
            i += 1
            continue

        if re.match(r"^\d+\.\s+", raw.lstrip()):
            text = md_inline_to_markup(re.sub(r"^\d+\.\s+", "", raw.lstrip()))
            story.append(Paragraph(text, styles["bullet"]))
            i += 1
            continue

        if raw.strip().startswith("*") and raw.strip().endswith("*") and not raw.strip().startswith("**"):
            inner = raw.strip().strip("*")
            story.append(Paragraph(f"<i>{md_inline_to_markup(inner)}</i>", styles["italic"]))
            i += 1
            continue

        story.append(Paragraph(md_inline_to_markup(raw.strip()), styles["body"]))
        i += 1

    doc.build(story)
    print(f"Wrote: {out_path}")


if __name__ == "__main__":
    main()
