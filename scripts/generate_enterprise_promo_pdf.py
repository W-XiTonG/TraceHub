#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Generate docs/TraceHub-企业采购与定制宣传.pdf"""

from __future__ import annotations

import sys
from pathlib import Path

from fpdf import FPDF

ROOT = Path(__file__).resolve().parents[1]
OUT_PDF = ROOT / "docs" / "TraceHub-企业采购与定制宣传.pdf"
FONT_PATH = Path(r"C:\Windows\Fonts\simhei.ttf")

SITE_URL = "http://182.92.159.29/"
API_BASE = "http://182.92.159.29"
DOCS_URL = "http://182.92.159.29/docs"
BUSINESS_PHONE = "15550151751"
FUTURE_SITE = "www.tracefuture.cn"
FUTURE_API = "api.tracefuture.cn"
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
        self.cell(0, 6, f"{BRAND}  |  企业采购方案  |  第 {self.page_no()} 页", align="C")

    def cover(self) -> None:
        self.add_page()
        self.set_font("SimHei", "B", 28)
        self.set_text_color(25, 55, 95)
        self.ln(28)
        self.multi_cell(0, 14, "TraceHub", align="C")
        self.set_font("SimHei", "B", 17)
        self.set_text_color(40, 40, 40)
        self.ln(3)
        self.multi_cell(0, 10, "企业采购与定制方案", align="C")
        self.ln(6)
        self.set_font("SimHei", "", 12)
        self.set_text_color(80, 80, 80)
        self.multi_cell(
            0,
            8,
            "可私有化部署  ·  可源码交付  ·  可品牌定制\n"
            "开源 AI API 网关全栈 —— 缩短自研周期，快速具备可运营能力",
            align="C",
        )
        self.ln(16)
        self.set_fill_color(240, 246, 252)
        self.set_font("SimHei", "B", 11)
        self.set_text_color(25, 55, 95)
        self.set_x(20)
        self.multi_cell(
            170,
            8,
            "不买轮子，买上线 —— 让企业更快拥有自己的 AI API 平台",
            align="C",
            fill=True,
        )
        self.ln(14)
        self.set_font("SimHei", "", 11)
        self.set_text_color(60, 60, 60)
        for line in [
            f"演示环境：{SITE_URL}",
            f"API 基址：{API_BASE}",
            f"技术文档：{DOCS_URL}",
            "",
            f"商务咨询：{BUSINESS_PHONE}（电话·微信同号）",
            "添加微信请备注：TraceHub 企业采购",
            f"正式域名（备案中）：{FUTURE_SITE} / {FUTURE_API}",
        ]:
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
        self.set_fill_color(240, 246, 252)
        self.set_font("SimHei", "B", 11)
        self.set_text_color(25, 55, 95)
        self.set_x(14)
        self.multi_cell(182, 7, text, fill=True)
        self.ln(3)

    def highlight(self, text: str) -> None:
        self.set_font("SimHei", "B", 10.5)
        self.set_text_color(0, 100, 60)
        self.multi_cell(0, 6, text)
        self.ln(1)

    def table_two_col(self, headers: tuple[str, str], rows: list[tuple[str, str]]) -> None:
        col_w = (52, 138)
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
            max_h = max(h1, self.get_y() - y)
            self.set_xy(x, y + max_h)
            self.cell(col_w[0], 0, "", border="LR")
            self.cell(col_w[1], 0, "", border="R", new_x="LMARGIN", new_y="NEXT")
            self.set_draw_color(200, 200, 200)
            self.line(x, y + max_h, x + col_w[0] + col_w[1], y + max_h)
            fill = not fill
        self.ln(3)

    def table_three_col(self, headers: tuple[str, str, str], rows: list[tuple[str, str, str]]) -> None:
        col_w = (38, 58, 94)
        self.set_font("SimHei", "B", 9)
        self.set_fill_color(25, 55, 95)
        self.set_text_color(255, 255, 255)
        for i, h in enumerate(headers):
            self.cell(col_w[i], 8, h, border=1, fill=True)
        self.ln()
        self.set_font("SimHei", "", 8.5)
        self.set_text_color(30, 30, 30)
        fill = False
        for row in rows:
            if self.get_y() > 262:
                self.add_page()
            if fill:
                self.set_fill_color(248, 248, 248)
            else:
                self.set_fill_color(255, 255, 255)
            y = self.get_y()
            x = self.get_x()
            heights = []
            for i, cell in enumerate(row):
                self.set_xy(x + sum(col_w[:i]), y)
                self.multi_cell(col_w[i], 5.5, cell, border="LTR", fill=True)
                heights.append(self.get_y() - y)
            max_h = max(heights)
            self.set_xy(x, y + max_h)
            for i in range(3):
                self.cell(col_w[i], 0, "", border="LR" if i == 0 else "R")
            self.ln(0)
            self.set_y(y + max_h)
            self.set_draw_color(200, 200, 200)
            self.line(x, y + max_h, x + sum(col_w), y + max_h)
            fill = not fill
        self.ln(3)


def build() -> Path:
    pdf = PromoPDF()
    pdf.cover()
    pdf.add_page()

    pdf.h1("产品定位（一句话）")
    pdf.quote_box(
        "可私有化部署、可二次开发、可品牌定制的开源 AI API 网关全栈产品。"
        "将多模型收敛为 OpenAI 兼容 /v1 接口，自带用户体系、计费运营、管理后台与营销官网。"
    )
    pdf.body(
        "帮助企业缩短 3～6 个月自研周期，快速具备可运营、可审计、可扩展的 AI 服务能力。"
        "适合行业云、集团 IT、AI 聚合平台、系统集成商等采购或合作定制。"
    )

    pdf.h1("为什么选择 TraceHub，而不是从零自研？")
    pdf.table_two_col(
        ("企业困境", "TraceHub 答案"),
        [
            ("自研网关周期长、人力贵", "成熟代码库 + 文档 + Docker 栈，数周 POC 到试点"),
            ("多模型协议不一", "统一 OpenAI 兼容出口，流式/工具调用网关对齐"),
            ("要做商业化缺运营骨架", "用户/Key/分组倍率/兑换码/日志已内置"),
            ("密钥与合规压力大", "AES 加密、Key 哈希、审计、私有化数据不出域"),
            ("需要自主品牌交付", "站点品牌化 + 源码交付 + 定制开发"),
        ],
    )
    pdf.highlight("把预算用在差异化业务与合规，而不是重复造路由+计费+控制台。")

    pdf.h1("采购后您得到什么？")
    pdf.h2("1. 可运行的产品（非 PPT）")
    for item in [
        "网关：对话、模型列表、Embeddings、图像、音频、视频、Realtime（按配置）",
        "路由：分组选路、权重/优先级、Failover、可选粘性会话",
        "计费：Token / 按次 / 混合；倍率策略；调用日志可对账",
        "用户端：注册登录、API Key、余额用量、兑换码、多语言控制台",
        "管理端：用户/渠道/分组/定价、日志、兑换码批量、审计、站点配置",
        "官网：Hero、模型展示、价格 FAQ（可白标替换）",
        "运维：/health、/ready、/metrics（Prometheus）",
    ]:
        pdf.bullet(item)

    pdf.h2("2. 可交付的工程资产")
    for item in [
        "完整源码（MIT，便于企业内长期演进）",
        "数据库模型与 Ent 迁移（PostgreSQL）",
        "Docker Compose 与单二进制托管 SPA 部署方案",
        "产品与开发文档（架构、API、数据库、运维）",
    ]:
        pdf.bullet(item)

    pdf.h2("3. 可选商务服务（循迹未来）")
    for item in [
        "私有化部署实施（内网 / 专有云 / 混合云）",
        "品牌与界面定制（Logo、域名、文案、主题）",
        "功能定制（支付、企业 SSO、RBAC、合规字段等）",
        "培训与维保（上线陪跑、升级、故障响应）",
    ]:
        pdf.bullet(item)

    pdf.add_page()
    pdf.h1("六大产品优势（决策者摘要）")
    advantages = [
        (
            "统一接入",
            "业务侧只对接一套 OpenAI 风格 API；多上游由网关消化，降低集成与供应商锁定。",
        ),
        (
            "可运营骨架",
            "不仅是代理，更是可对外/对内运营的平台：用户、Key、余额、兑换码、分组定价。",
        ),
        (
            "调度与计费清晰",
            "Failover、多通道池化、Token/按次计费、日志对账；财务/运维/研发规则一致。",
        ),
        (
            "安全与内控",
            "渠道密钥 AES 加密、用户 Key 哈希、审计日志；支持私有化与数据不出域。",
        ),
        (
            "生产级运维",
            "Go 高并发、无状态扩展、Prometheus 指标；纳入企业监控与发布体系。",
        ),
        (
            "开源 + 定制",
            "MIT 可审计可 fork；支付/OAuth/RBAC/Webhook 等可按项目单独立项。",
        ),
    ]
    for title, desc in advantages:
        pdf.h2(title)
        pdf.body(desc)

    pdf.h1("典型客户场景")
    pdf.table_three_col(
        ("客户类型", "典型诉求", "TraceHub 价值"),
        [
            ("行业云 / 数服商", "快速上线大模型 API 产品线", "全栈 + 白标，缩短上市"),
            ("集团 IT / 研发中心", "内网统一 AI 出口", "私有化 + 分组计费 + 审计"),
            ("AI 创业 / 聚合平台", "可运营、可扩展", "运营骨架现成，专注获客"),
            ("系统集成商", "可交付可维保标品", "源码 + 实施 + 定制，提升毛利"),
        ],
    )

    pdf.h1("合作模式（灵活可选）")
    pdf.table_three_col(
        ("模式", "说明", "适合"),
        [
            ("开源自用", "MIT 自行部署二次开发", "有成熟研发团队"),
            ("企业授权+源码包", "含部署指导与商务支持", "需清晰采购流程"),
            ("私有化部署实施", "规划/安装/调优/验收", "内网/等保/专有云"),
            ("定制开发", "按需求清单里程碑交付", "支付/SSO/合规等"),
            ("年度维保", "升级巡检故障响应", "生产长期稳定"),
        ],
    )
    pdf.body("具体报价、授权范围与 SLA 以双方商务合同为准。")

    pdf.add_page()
    pdf.h1("可定制方向（示例）")
    for item in [
        "品牌与前台：Logo、主题、域名、官网文案、多语言",
        "支付与账务：支付宝/微信/Stripe、对账导出",
        "企业身份：LDAP/OIDC/企微/钉钉 SSO；细粒度 RBAC",
        "安全合规：IP 白名单、日志脱敏、保留周期、等保对标",
        "路由与 SLA：专属 Failover、渠道探测、成本/延迟优先",
        "协议与模型：新上游类型、Realtime 增强、行业模型映射",
        "集成：Webhook、对接现有计费/CRM/工单",
    ]:
        pdf.bullet(item)
    pdf.body(
        "建议首次沟通提供：目标用户规模、部署环境、必接模型、合规要求、上线时间。"
        "便于 1～2 个工作日内给出粗估方案。"
    )

    pdf.h1("技术架构（采购评审摘要）")
    pdf.table_two_col(
        ("维度", "说明"),
        [
            ("后端", "Go 1.25+、Gin、Ent ORM、PostgreSQL"),
            ("缓存", "Redis 7+（可选，无则部分能力降级）"),
            ("前端", "Vue 3、Vite、Tailwind CSS v4"),
            ("部署", "Docker Compose；单进程托管 SPA"),
            ("许可", "MIT（可读、可改、可私有化）"),
        ],
    )

    pdf.h1("建议交付路径")
    pdf.body(
        "需求沟通 → 演示/POC（Docker 全链路）→ 方案与报价 → 合同 "
        "→ 部署实施或源码交付 → 定制开发（可选）→ 验收培训 → 维保"
    )
    pdf.h2("POC 验收建议项")
    for item in [
        "用户注册与登录",
        "创建 API Key 并成功调用对话（含流式）",
        "管理端配置渠道与模型倍率",
        "查看调用日志与扣费记录",
    ]:
        pdf.bullet(item)

    pdf.h1("立即联系 · 循迹未来")
    pdf.set_fill_color(25, 55, 95)
    pdf.set_text_color(255, 255, 255)
    pdf.set_font("SimHei", "B", 12)
    pdf.set_x(14)
    pdf.multi_cell(
        182,
        8,
        f"商务电话 / 微信：{BUSINESS_PHONE}\n"
        f"演示环境：{SITE_URL}\n"
        f"添加微信请备注：TraceHub 企业采购",
        fill=True,
        align="C",
    )
    pdf.ln(4)
    pdf.set_text_color(30, 30, 30)
    pdf.h2("宣传语")
    pdf.quote_box("不买轮子，买上线 —— TraceHub 让企业更快拥有自己的 AI API 平台。")
    pdf.body("开源可审计 · 私有化可交付 · 品牌可定制 · 运营开箱即用")

    pdf.ln(2)
    pdf.set_font("SimHei", "", 9)
    pdf.set_text_color(100, 100, 100)
    pdf.multi_cell(
        0,
        5,
        "产品能力以开源仓库 README 与双方合同约定为准；规划能力见路线图。\n"
        "生成脚本：scripts/generate_enterprise_promo_pdf.py",
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
