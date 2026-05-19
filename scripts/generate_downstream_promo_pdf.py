#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Generate docs/TraceHub-下游优势宣传.pdf from structured content."""

from __future__ import annotations

import sys
from pathlib import Path

from fpdf import FPDF

ROOT = Path(__file__).resolve().parents[1]
OUT_PDF = ROOT / "docs" / "TraceHub-下游优势宣传.pdf"
FONT_PATH = Path(r"C:\Windows\Fonts\simhei.ttf")

# Brand / URLs (align with docs/_site-urls.yaml)
SITE_URL = "http://182.92.159.29/"
API_BASE = "http://182.92.159.29"
DOCS_URL = "http://182.92.159.29/docs"
BRAND = "迹径 · TraceHub — 循迹未来"


class PromoPDF(FPDF):
    def __init__(self) -> None:
        super().__init__(orientation="P", unit="mm", format="A4")
        self.set_auto_page_break(auto=True, margin=18)
        if not FONT_PATH.is_file():
            raise FileNotFoundError(f"Chinese font not found: {FONT_PATH}")
        self.add_font("SimHei", "", str(FONT_PATH))
        self.add_font("SimHei", "B", str(FONT_PATH))

    def footer(self) -> None:
        self.set_y(-12)
        self.set_font("SimHei", "", 8)
        self.set_text_color(120, 120, 120)
        self.cell(0, 6, f"{BRAND}  |  第 {self.page_no()} 页", align="C")

    def cover(self) -> None:
        self.add_page()
        self.set_font("SimHei", "B", 26)
        self.set_text_color(25, 55, 95)
        self.ln(35)
        self.multi_cell(0, 14, "TraceHub", align="C")
        self.set_font("SimHei", "B", 18)
        self.set_text_color(40, 40, 40)
        self.ln(4)
        self.multi_cell(0, 10, "面向下游（调用方）的核心优势", align="C")
        self.ln(8)
        self.set_font("SimHei", "", 12)
        self.set_text_color(80, 80, 80)
        self.multi_cell(
            0,
            8,
            "OpenAI 兼容 · 一次对接 · 多模型统一出口\n少写集成 · 少背运维 · 成本可算 · 协议不变",
            align="C",
        )
        self.ln(20)
        self.set_font("SimHei", "", 11)
        self.set_text_color(60, 60, 60)
        lines = [
            f"官网 / 控制台：{SITE_URL}",
            f"API Base URL：{API_BASE}",
            f"接入文档：{DOCS_URL}",
            "",
            "适用对象：应用开发者 · Agent 平台 · SaaS · 企业内部 AI 出口",
        ]
        for line in lines:
            self.cell(0, 7, line, align="C", new_x="LMARGIN", new_y="NEXT")

    def h1(self, text: str) -> None:
        self.ln(4)
        self.set_font("SimHei", "B", 15)
        self.set_text_color(25, 55, 95)
        self.multi_cell(0, 9, text)
        self.set_draw_color(25, 55, 95)
        self.line(10, self.get_y(), 200, self.get_y())
        self.ln(3)

    def h2(self, text: str) -> None:
        self.ln(2)
        self.set_font("SimHei", "B", 12)
        self.set_text_color(50, 50, 50)
        self.multi_cell(0, 7, text)
        self.ln(1)

    def body(self, text: str) -> None:
        self.set_font("SimHei", "", 10.5)
        self.set_text_color(30, 30, 30)
        self.multi_cell(0, 6, text)
        self.ln(1)

    def bullet(self, text: str) -> None:
        self.set_font("SimHei", "", 10.5)
        self.set_text_color(30, 30, 30)
        x = self.get_x()
        self.cell(6, 6, "-")
        self.multi_cell(0, 6, text)
        self.set_x(x)

    def quote_box(self, text: str) -> None:
        y0 = self.get_y()
        self.set_fill_color(240, 246, 252)
        self.set_font("SimHei", "B", 11)
        self.set_text_color(25, 55, 95)
        self.set_x(14)
        self.multi_cell(182, 7, text, fill=True)
        self.ln(2)
        if self.get_y() - y0 < 10:
            self.ln(2)

    def result_line(self, text: str) -> None:
        self.set_font("SimHei", "B", 10.5)
        self.set_text_color(0, 100, 60)
        self.multi_cell(0, 6, text)
        self.ln(1)

    def table_two_col(self, headers: tuple[str, str], rows: list[tuple[str, str]]) -> None:
        col_w = (48, 142)
        self.set_font("SimHei", "B", 10)
        self.set_fill_color(25, 55, 95)
        self.set_text_color(255, 255, 255)
        self.cell(col_w[0], 8, headers[0], border=1, fill=True)
        self.cell(col_w[1], 8, headers[1], border=1, fill=True, new_x="LMARGIN", new_y="NEXT")
        self.set_font("SimHei", "", 9.5)
        self.set_text_color(30, 30, 30)
        fill = False
        for a, b in rows:
            if self.get_y() > 265:
                self.add_page()
            if fill:
                self.set_fill_color(248, 248, 248)
            else:
                self.set_fill_color(255, 255, 255)
            y = self.get_y()
            x = self.get_x()
            self.multi_cell(col_w[0], 6, a, border="LTR", fill=True)
            h1 = self.get_y() - y
            self.set_xy(x + col_w[0], y)
            self.multi_cell(col_w[1], 6, b, border="TR", fill=True)
            h2 = self.get_y() - y
            max_h = max(h1, h2)
            self.set_xy(x, y + max_h)
            self.cell(col_w[0], 0, "", border="LR")
            self.cell(col_w[1], 0, "", border="R", new_x="LMARGIN", new_y="NEXT")
            self.set_draw_color(200, 200, 200)
            self.line(x, y + max_h, x + col_w[0] + col_w[1], y + max_h)
            fill = not fill
        self.ln(3)


def build() -> Path:
    pdf = PromoPDF()
    pdf.cover()
    pdf.add_page()

    pdf.h1("核心价值（一句话）")
    pdf.quote_box(
        "只改 Base URL 和 API Key，原有 OpenAI SDK / 客户端继续用；"
        "背后多模型、多线路、路由与计费由网关代劳。"
    )
    pdf.body(
        "迹径 TraceHub 是 OpenAI 兼容的 AI API 统一网关。"
        "对下游（应用、Agent、SaaS、内部工具）而言：少写集成、少背运维、成本可算、协议不变、能力可扩。"
    )

    pdf.h1("1. 接入成本极低：一套协议走天下")
    pdf.table_two_col(
        ("下游痛点", "TraceHub 给什么"),
        [
            (
                "每家模型一套 SDK、鉴权、流式格式",
                "统一 /v1 OpenAI 风格接口；对话、模型列表与生态习惯一致",
            ),
            (
                "换模型要改代码",
                "改 model 参数即可；模型别名对外稳定、背后换线透明",
            ),
            (
                "从 OpenAI 迁出成本高",
                "迁移 ≈ 两行配置：baseURL + apiKey；LangChain 等多数可直接复用",
            ),
        ],
    )
    pdf.result_line("结果：精力花在产品与 Prompt，而非 N 家厂商 × M 套适配器。")

    pdf.h1("2. 调用体验稳定：流式、工具调用、多模态")
    for item in [
        "SSE 流式完整支持；多上游统一为 OpenAI 风格 chat.completion.chunk，客户端无需按厂商分叉解析。",
        "能力矩阵（以配置为准）：对话、补全、Embeddings、图像、音频、视频、Realtime 等同一 /v1 体系扩展。",
        "工具调用 / 多模态按 OpenAI 兼容请求体传递，无需为每条线路单独造轮子。",
    ]:
        pdf.bullet(item)
    pdf.result_line("结果：Agent、Copilot、客服机器人协议层一次定型，模型选型变成配置问题。")

    pdf.h1("3. 可靠性由网关兜底")
    for item in [
        "健康准入：异常线路自动退出选路池。",
        "自动故障转移：网络失败、5xx、429 等可换渠道重试（有上限；流式已输出不再切换）。",
        "多通道池化 + 权重/优先级：分散压力或按「先 A 后 B」运维。",
        "粘性会话（可选）：长对话同会话固定线路，失效后回退普通选路。",
    ]:
        pdf.bullet(item)
    pdf.result_line("结果：业务只打一个稳定入口，线路抖动由网关吸收。")

    pdf.h1("4. 成本可预期、可对账")
    for item in [
        "按 Token 精确计量：输入/输出分开计价，规则控制台可对齐说明。",
        "灵活倍率：分组、用户、单模型维度，支持普通价 / 会员价 / 内测价。",
        "按次计费与混合模式：适合图像、视频、固定单价类 API。",
        "免计费模型（可配置）：联调灰度可不扣额度，仍可记日志。",
        "调用日志：时间、模型、Token、费用、状态码可追溯，便于对账与排障。",
    ]:
        pdf.bullet(item)
    pdf.result_line("结果：财务与研发用同一套数据，避免账单对不上。")

    pdf.add_page()
    pdf.h1("5. 安全边界清晰")
    for item in [
        "下游只持平台 API Key；上游渠道密钥 AES 加密存服务端，不进仓库与客户端。",
        "用户 Key 哈希保存；管理端敏感操作可审计留痕。",
        "可拦截云端控制台类 /v1 路径，降低账号管理 API 误暴露风险。",
        "可选：邮件验证码、图形验证码、/metrics Bearer 保护（依赖运营配置）。",
    ]:
        pdf.bullet(item)
    pdf.result_line("结果：适合多租户 SaaS 与企业统一出口，应用侧攻击面更小。")

    pdf.h1("6. 可观测、可扩容")
    for item in [
        "/health、/ready：K8s 探针友好。",
        "/metrics（Prometheus）：QPS、延迟、错误率接入现有监控。",
        "无状态水平扩展：加实例 + 负载均衡即可。",
        "Go 高并发转发，面向 API 低延迟场景优化。",
    ]:
        pdf.bullet(item)
    pdf.result_line("结果：当作标准基础设施组件接入，而非黑盒脚本。")

    pdf.h1("7. 控制台与生态")
    for item in [
        "官网 + 用户控制台：注册、Key、余额用量、兑换码、改密自助闭环。",
        "多语言：简中 / 繁中 / English。",
        "分组默认模型、CORS：减少 Web/H5 漏传 model 失败。",
        "开源 MIT + Docker Compose：托管或私有化/二次品牌化均可。",
    ]:
        pdf.bullet(item)
    pdf.result_line("结果：从个人试调到企业内网网关，同一产品骨架可承接。")

    pdf.h1("8. 谁最适合接入？")
    for item in [
        "产品 / 研发：快速 A/B 多模型，不想维护多套集成。",
        "Agent / 自动化：依赖流式 + Tool Calls + 稳定 Base URL。",
        "SaaS：统一出口与计量，AI 成本摊到租户或项目。",
        "初创 / 项目制：可预期单价 + 可导出日志，支撑报价与复盘。",
    ]:
        pdf.bullet(item)

    pdf.h1("9. 三步开始")
    for i, step in enumerate(
        [
            "注册 → 控制台创建 API Key（仅创建时完整展示，请妥善保存）。",
            f"将客户端 baseURL 指向网关（当前示例：{API_BASE}，路径以文档为准）。",
            "Authorization: Bearer <平台 Key>，设置 model，即可联调。",
        ],
        1,
    ):
        pdf.body(f"{i}. {step}")
    pdf.body("余额不足可能返回 HTTP 402，便于程序侧引导充值。")

    pdf.h1("宣传语（海报 / 落地页）")
    pdf.h2("主标语")
    pdf.quote_box("多模型，一条链路 —— OpenAI 兼容，一次接入。")
    pdf.h2("副标语（任选）")
    for s in [
        "改两行配置，不换开发习惯。",
        "路由、Failover、计费、日志，网关全包，你只写业务。",
        "Token 透明、按次可配、日志可对账，AI 成本看得见。",
        "流式统一、工具调用即用，Agent 上线更快。",
        "开源可私有化，托管可即开即用。",
    ]:
        pdf.bullet(s)

    pdf.h2("电梯演讲（约 30 秒）")
    pdf.body(
        "TraceHub 把多家大模型收敛成一套 OpenAI 兼容 API。"
        "下游换 Base URL 和 Key，SDK 照旧；换模型只改参数。"
        "网关负责多线路调度、故障转移、流式对齐、Token/按次计费与调用日志，"
        "并提供控制台与可选私有化。团队专注产品，集成与运维交给平台。"
    )

    pdf.h1("说明边界")
    pdf.body("以下依赖运营配置与部署版本，对外请注明「以实际上线为准」：")
    pdf.bullet("Failover 次数、粘性会话、透传白名单、Realtime、免计费模型名单。")
    pdf.bullet("在线支付、OAuth、细粒度 RBAC、语义缓存等见路线图，非默认必交付。")
    pdf.ln(4)
    pdf.set_font("SimHei", "", 9)
    pdf.set_text_color(100, 100, 100)
    pdf.multi_cell(
        0,
        5,
        "完整能力见仓库 README.zh-CN.md、docs/产品文档.md、docs/TraceHub-客户接入宣传.md\n"
        "本文档生成：scripts/generate_downstream_promo_pdf.py",
    )

    OUT_PDF.parent.mkdir(parents=True, exist_ok=True)
    pdf.output(str(OUT_PDF))
    return OUT_PDF


if __name__ == "__main__":
    try:
        path = build()
        print(f"OK: {path}")
    except Exception as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)
