const { Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell,
        Header, Footer, AlignmentType, PageOrientation, LevelFormat,
        HeadingLevel, BorderStyle, WidthType, ShadingType,
        VerticalAlign, PageNumber, PageBreak } = require('docx');
const fs = require('fs');

// 合同日期
const contractDate = new Date().toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' });

// 表格边框样式
const border = { style: BorderStyle.SINGLE, size: 1, color: "000000" };
const borders = { top: border, bottom: border, left: border, right: border };

// 创建标题段落
function createTitle(text) {
    return new Paragraph({
        alignment: AlignmentType.CENTER,
        spacing: { after: 200 },
        children: [new TextRun({ text, bold: true, size: 44, font: "SimSun" })]
    });
}

function createHeading1(text) {
    return new Paragraph({
        heading: HeadingLevel.HEADING_1,
        spacing: { before: 400, after: 200 },
        children: [new TextRun({ text, bold: true, size: 28, font: "SimSun" })]
    });
}

function createHeading2(text) {
    return new Paragraph({
        heading: HeadingLevel.HEADING_2,
        spacing: { before: 300, after: 150 },
        children: [new TextRun({ text, bold: true, size: 24, font: "SimSun" })]
    });
}

function createParagraph(text, options = {}) {
    return new Paragraph({
        alignment: options.align || AlignmentType.JUSTIFIED,
        indent: options.indent ? { firstLine: 480 } : undefined,
        spacing: { after: options.after || 120, line: 360 },
        children: [new TextRun({ text, size: 22, font: "SimSun" })]
    });
}

function createEmptyLine() {
    return new Paragraph({ children: [] });
}

// 创建信息表格行
function createInfoRow(label, value) {
    return new TableRow({
        children: [
            new TableCell({
                borders,
                width: { size: 2400, type: WidthType.DXA },
                shading: { fill: "F0F0F0", type: ShadingType.CLEAR },
                margins: { top: 80, bottom: 80, left: 120, right: 120 },
                verticalAlign: VerticalAlign.CENTER,
                children: [new Paragraph({ children: [new TextRun({ text: label, size: 22, font: "SimSun" })] })]
            }),
            new TableCell({
                borders,
                width: { size: 6960, type: WidthType.DXA },
                margins: { top: 80, bottom: 80, left: 120, right: 120 },
                children: [new Paragraph({ children: [new TextRun({ text: value, size: 22, font: "SimSun" })] })]
            })
        ]
    });
}

// 创建签字区域
function createSignatureArea(title, name) {
    return new Paragraph({
        spacing: { before: 300, after: 100 },
        children: [
            new TextRun({ text: `${title}：`, size: 22, font: "SimSun" }),
            new TextRun({ text: "\t\t\t\t\t", size: 22 }),
            new TextRun({ text: `（${name}）`, size: 22, font: "SimSun" })
        ]
    });
}

const doc = new Document({
    styles: {
        default: { document: { run: { font: "SimSun", size: 22 } } },
        paragraphStyles: [
            { id: "Heading1", name: "Heading 1", basedOn: "Normal", next: "Normal", quickFormat: true,
              run: { size: 28, bold: true, font: "SimSun" },
              paragraph: { spacing: { before: 400, after: 200 }, outlineLevel: 0 } },
            { id: "Heading2", name: "Heading 2", basedOn: "Normal", next: "Normal", quickFormat: true,
              run: { size: 24, bold: true, font: "SimSun" },
              paragraph: { spacing: { before: 300, after: 150 }, outlineLevel: 1 } }
        ]
    },
    numbering: {
        config: [
            { reference: "article-numbers",
              levels: [{ level: 0, format: LevelFormat.DECIMAL, text: "第%1条", alignment: AlignmentType.LEFT,
                style: { paragraph: { indent: { left: 480, hanging: 480 } } } }] },
            { reference: "sub-numbers",
              levels: [{ level: 0, format: LevelFormat.DECIMAL, text: "%1.", alignment: AlignmentType.LEFT,
                style: { paragraph: { indent: { left: 720, hanging: 360 } } } }] },
            { reference: "bullets",
              levels: [{ level: 0, format: LevelFormat.BULLET, text: "•", alignment: AlignmentType.LEFT,
                style: { paragraph: { indent: { left: 720, hanging: 360 } } } }] }
        ]
    },
    sections: [{
        properties: {
            page: {
                size: { width: 11906, height: 16838 }, // A4
                margin: { top: 1440, right: 1440, bottom: 1440, left: 1440 }
            }
        },
        headers: {
            default: new Header({
                children: [new Paragraph({
                    alignment: AlignmentType.CENTER,
                    children: [new TextRun({ text: "TraceHub 软件产品销售及定制开发服务合同", size: 18, font: "SimSun", color: "808080" })]
                })]
            })
        },
        footers: {
            default: new Footer({
                children: [new Paragraph({
                    alignment: AlignmentType.CENTER,
                    children: [new TextRun({ text: "第 ", size: 18, font: "SimSun" }), 
                              new TextRun({ children: [PageNumber.CURRENT], size: 18 }), 
                              new TextRun({ text: " 页 / 共 ", size: 18, font: "SimSun" }), 
                              new TextRun({ children: [PageNumber.TOTAL_PAGES], size: 18 }),
                              new TextRun({ text: " 页", size: 18, font: "SimSun" })]
                })]
            })
        },
        children: [
            // ==================== 封面 ====================
            createEmptyLine(), createEmptyLine(), createEmptyLine(),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                spacing: { after: 600 },
                children: [new TextRun({ text: "软 件 产 品 销 售 及", bold: true, size: 52, font: "SimHei" })]
            }),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                spacing: { after: 800 },
                children: [new TextRun({ text: "定 制 开 发 服 务 合 同", bold: true, size: 52, font: "SimHei" })]
            }),
            createEmptyLine(),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                spacing: { after: 200 },
                children: [new TextRun({ text: "【产品名称：TraceHub AI API 智能网关系统】", size: 28, font: "SimSun", color: "2E75B6" })]
            }),
            createEmptyLine(), createEmptyLine(), createEmptyLine(), createEmptyLine(),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                spacing: { after: 100 },
                children: [new TextRun({ text: "甲方（委托方）：____________________", size: 24, font: "SimSun" })]
            }),
            createEmptyLine(),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                spacing: { after: 100 },
                children: [new TextRun({ text: "乙方（开发方）：循迹未来团队", size: 24, font: "SimSun" })]
            }),
            createEmptyLine(), createEmptyLine(), createEmptyLine(), createEmptyLine(),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                spacing: { after: 100 },
                children: [new TextRun({ text: `签订日期：${contractDate}`, size: 24, font: "SimSun" })]
            }),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                spacing: { after: 100 },
                children: [new TextRun({ text: "签订地点：____________________", size: 24, font: "SimSun" })]
            }),

            // ==================== 分页：正文开始 ====================
            new Paragraph({ children: [new PageBreak()] }),

            // ==================== 第一条：定义与说明 ====================
            createHeading1("第一条  定义与说明"),
            createParagraph("本合同中的下列术语，除非上下文另有约定，应具有如下含义："),
            createEmptyLine(),

            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: `"软件产品"`, bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: `：指乙方自主研发并拥有知识产权的「TraceHub AI API 智能网关系统」（以下简称"TraceHub"），包括但不限于后端网关程序、前端管理界面、数据库脚本、配置文件、部署文档及相关技术资料。`, size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: '"标准交付版"', bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "：指乙方向甲方交付的 TraceHub 当前版本（V1.0.0 及以上）的标准功能集合，具体范围以本合同第二条为准。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: '"定制开发"', bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "：指根据甲方的特定需求，在标准交付版基础上进行的额外功能开发、接口对接、界面定制或系统集成等服务。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: '"部署环境"', bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "：指甲方提供的用于安装运行 TraceHub 的服务器硬件、操作系统、网络环境及配套基础设施。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: '"验收"', bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "：指甲方依据本合同约定的技术指标和功能清单对交付成果进行测试和确认的过程。", size: 22, font: "SimSun" })
                ]
            }),
            createEmptyLine(),

            // ==================== 第二条：产品概述与功能范围 ====================
            createHeading1("第二条  产品概述与功能范围"),

            createHeading2("2.1 产品定位"),
            createParagraph("TraceHub 是一套企业级 AI API 网关与资产管理系统，旨在为企业和开发者提供一站式的多模型 AI 服务聚合方案。系统将来自 OpenAI、Claude（Anthropic）、Gemini（Google）、DeepSeek、阿里百炼等多种大模型服务商的 API 以统一的 OpenAI 兼容格式对外暴露，并提供完善的用户管理、API Key 管理、智能路由调度、精确计费、运营后台等能力。", { indent: true }),

            createHeading2("2.2 技术架构"),
            new Table({
                width: { size: 9026, type: WidthType.DXA },
                columnWidths: [2400, 6626],
                rows: [
                    new TableRow({ children: [
                        new TableCell({ borders, shading: { fill: "E7E6E6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "技术层级", bold: true, size: 22, font: "SimSun" })] })] }),
                        new TableCell({ borders, shading: { fill: "E7E6E6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "技术选型", bold: true, size: 22, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "后端框架", size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Go 1.25+ / Gin Web框架 / Ent ORM", size: 21, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "前端框架", size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Vue 3 / Vite / Tailwind CSS v4 / Pinia", size: 21, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "数据存储", size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "PostgreSQL 16+（主库）/ Redis 7+（缓存与会话）", size: 21, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "部署方式", size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Docker Compose 一键部署 / 单二进制 + SPA 托管 / Kubernetes 编排", size: 21, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "监控运维", size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Prometheus 指标 / 健康检查探针 / 结构化日志（Zap）", size: 21, font: "SimSun" })] })] })
                    ]})
                ]
            }),
            createEmptyLine(),

            createHeading2("2.3 标准交付版功能清单"),
            createParagraph("标准交付版包含以下已成熟可用的核心功能模块（V1.0.0 基线）：", { indent: true }),
            createEmptyLine(),

            // 功能模块表格
            new Table({
                width: { size: 9026, type: WidthType.DXA },
                columnWidths: [1800, 3600, 3626],
                rows: [
                    new TableRow({ children: [
                        new TableCell({ borders, shading: { fill: "2E75B6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "功能类别", bold: true, size: 21, font: "SimSun", color: "FFFFFF" })] })] }),
                        new TableCell({ borders, shading: { fill: "2E75B6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "功能项", bold: true, size: 21, font: "SimSun", color: "FFFFFF" })] })] }),
                        new TableCell({ borders, shading: { fill: "2E75B6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "功能描述", bold: true, size: 21, font: "SimSun", color: "FFFFFF" })] })] })
                    ]}),
                    // 网关核心
                    new TableRow({ children: [
                        new TableCell({ borders, rowSpan: 8, verticalAlign: VerticalAlign.CENTER, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "网关核心", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "对话补全 API", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "POST /v1/chat/completions，支持 SSE 流式输出", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "模型列表 API", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "GET /v1/models，返回用户可用模型列表", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "向量嵌入 API", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "POST /v1/embeddings，文本向量化", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "图像生成 API", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "POST /v1/images/generations，AI 图像生成", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "视频生成 API", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "POST/GET/DELETE /v1/videos*，含火山引擎等扩展", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "音频处理 API", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "TTS 语音合成、语音转写、翻译", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Realtime 通话", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "实时语音通话子资源映射（可选开启）", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "透传扩展", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "白名单控制下透传更多 /v1/* 路径", size: 20, font: "SimSun" })] })] })
                    ]}),
                    // 智能路由
                    new TableRow({ children: [
                        new TableCell({ borders, rowSpan: 5, verticalAlign: VerticalAlign.CENTER, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "智能路由调度", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "分组+模型选路", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "按用户分组与模型筛选可用渠道", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "权重/优先级路由", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "加权随机分配 + 固定优先顺序", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "故障自动转移", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "5xx/429/超时自动切换备用渠道", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "粘性会话", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Redis 支持同一会话固定路由到同一线路", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "健康探测治理", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "自动检测渠道可用性，异常线路暂不参与选路", size: 20, font: "SimSun" })] })] })
                    ]}),
                    // 计费引擎
                    new TableRow({ children: [
                        new TableCell({ borders, rowSpan: 4, verticalAlign: VerticalAlign.CENTER, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "计费引擎", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Token 精确统计", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "基于 tiktoken 分词器按模型精确计算 Token", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "多维度倍率", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "全局/分组/用户/模型四级倍率体系", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "免计费模型", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "可配置免计费模型名单（仍记录日志）", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "额度扣减", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "预扣 + 实际用量修正的双阶段计费机制", size: 20, font: "SimSun" })] })] })
                    ]}),
                    // 用户体系
                    new TableRow({ children: [
                        new TableCell({ borders, rowSpan: 5, verticalAlign: VerticalAlign.CENTER, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "用户体系", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "注册登录认证", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "邮箱/验证码登录，JWT Token 鉴权", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "API Key 管理", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Key 生成/删除/重命名，哈希安全存储", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "余额与用量", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "实时余额查询、调用历史、消费明细", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "兑换码", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "兑换码领取额度，支持批量生成与管理", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "图形验证码", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "登录人机校验，防止恶意注册与撞库", size: 20, font: "SimSun" })] })] })
                    ]}),
                    // 管理后台
                    new TableRow({ children: [
                        new TableCell({ borders, rowSpan: 6, verticalAlign: VerticalAlign.CENTER, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "管理后台", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "用户管理", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "用户列表/搜索/禁用启用/手动调整额度", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "渠道管理", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "50+ 种上游通道类型，AES-GCM 加密存储凭据", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "分组与倍率", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "用户分组管理、模型级输入/输出倍率配置", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "调用日志", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "全量调用记录，支持按用户/模型/状态筛选", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "审计日志", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "管理员操作完整留痕，满足内控溯源需求", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "站点品牌配置", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "站点名称、Logo、联系方式等自定义配置", size: 20, font: "SimSun" })] })] })
                    ]}),
                    // 前端界面
                    new TableRow({ children: [
                        new TableCell({ borders, rowSpan: 3, verticalAlign: VerticalAlign.CENTER, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "前端界面", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "营销官网", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Hero/模型展示/价格表/FAQ/多语言(中繁英)", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "用户控制台", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "仪表盘/API Key 管理/余额/日志/设置", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "站内文档中心", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "内置 /docs 文档页面（简中/繁中/English）", size: 20, font: "SimSun" })] })] })
                    ]}),
                    // 运维支撑
                    new TableRow({ children: [
                        new TableCell({ borders, rowSpan: 3, verticalAlign: VerticalAlign.CENTER, margins: { top: 60, bottom: 60, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "运维支撑", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "健康检查", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "/health、/ready 探针，支持 K8s 就绪检查", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Prometheus 指标", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "/metrics 端点，QPS/延迟/错误率等核心指标", size: 20, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "容器化部署", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "Docker Compose 一键编排，单二进制 + SPA 托管", size: 20, font: "SimSun" })] })] })
                    ]})
                ]
            }),
            createEmptyLine(),

            // ==================== 第三条：合作模式与交付方式 ====================
            createHeading1("第三条  合作模式与交付方式"),

            createHeading2("3.1 合作模式"),
            createParagraph("本合同提供以下两种合作方式，甲方可在签署时选择其中一种或两种组合：", { indent: true }),
            createEmptyLine(),

            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: "【模式A — 标准产品授权】", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "：甲方采购 TraceHub 标准交付版的永久使用权。乙方提供完整的软件源代码（MIT 开源协议）、二进制文件、部署文档及技术培训。甲方获得在自有服务器上部署、使用和二次开发的完整权利。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: "【模式B — 定制开发服务】", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "：甲方提出超出标准交付范围的定制需求，乙方根据需求评估后提供定制开发报价与工期。定制内容包括但不限于：新功能模块开发、第三方系统集成、UI/UX 品牌定制、特殊协议适配、性能优化调优等。", size: 22, font: "SimSun" })
                ]
            }),
            createEmptyLine(),

            createHeading2("3.2 标准版交付物清单"),
            new Table({
                width: { size: 9026, type: WidthType.DXA },
                columnWidths: [600, 2800, 5626],
                rows: [
                    new TableRow({ children: [
                        new TableCell({ borders, shading: { fill: "E7E6E6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "序号", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, shading: { fill: "E7E6E6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "交付物名称", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, shading: { fill: "E7E6E6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "说明", bold: true, size: 21, font: "SimSun" })] })] })
                    ]}),
                    ...[
                        ["1", "源代码包", "完整的 Go 后端 + Vue 前端源代码，含 git 版本历史"],
                        ["2", "编译产物", "预编译的可执行文件（Linux amd64/arm64）、前端静态资源"],
                        ["3", "数据库迁移脚本", "Ent ORM 自动迁移脚本与种子数据"],
                        ["4", "配置文件模板", "生产级 YAML 配置模板（含详细注释说明）"],
                        ["5", "Docker 部署包", "Dockerfile + docker-compose.yml 一键部署编排文件"],
                        ["6", "部署与运维手册", "环境准备、安装步骤、配置指南、常见问题排查"],
                        ["7", "API 接口文档", "OpenAI 兼容端点说明、请求/响应示例、错误码对照表"],
                        ["8", "管理员操作手册", "管理后台各功能模块的操作指引"],
                        ["9", "在线支付集成指导", "支付宝/微信/Stripe 对接参考代码（需自行申请商户资质）"],
                        ["10", "技术培训（可选）", "线上或线下部署培训与架构讲解（另行约定时长）"]
                    ].map((row, i) => new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 120, right: 120 }, verticalAlign: VerticalAlign.CENTER,
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: row[0], size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: row[1], bold: true, size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: row[2], size: 20, font: "SimSun" })] })] })
                    ]}))
                ]
            }),
            createEmptyLine(),

            // ==================== 第四条：费用与付款方式 ====================
            createHeading1("第四条  费用与付款方式"),

            createHeading2("4.1 费用构成"),
            new Table({
                width: { size: 9026, type: WidthType.DXA },
                columnWidths: [2600, 2200, 2226, 2000],
                rows: [
                    new TableRow({ children: [
                        new TableCell({ borders, shading: { fill: "2E75B6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "项目", bold: true, size: 21, font: "SimSun", color: "FFFFFF" })] })] }),
                        new TableCell({ borders, shading: { fill: "2E75B6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "费用（人民币）", bold: true, size: 21, font: "SimSun", color: "FFFFFF" })] })] }),
                        new TableCell({ borders, shading: { fill: "2E75B6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "备注", bold: true, size: 21, font: "SimSun", color: "FFFFFF" })] })] }),
                        new TableCell({ borders, shading: { fill: "2E75B6", type: ShadingType.CLEAR }, margins: { top: 80, bottom: 80, left: 120, right: 120 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "付款节点", bold: true, size: 21, font: "SimSun", color: "FFFFFF" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "标准版授权费（模式A）", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "￥__________", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "含源代码+文档+基础培训", size: 19, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "签约后3日内付50%", size: 19, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "验收通过后3日内付50%", size: 19, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "定制开发费（模式B）", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "￥__________", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "详见附件《定制需求说明书》", size: 19, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "分期支付（见附件）", size: 19, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "年度维保服务（可选）", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "￥__________/年", size: 20, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "含Bug修复+安全补丁+版本升级", size: 19, font: "SimSun" })] })] }),
                        new TableCell({ borders, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "按年预付", size: 19, font: "SimSun" })] })] })
                    ]}),
                    new TableRow({ children: [
                        new TableCell({ borders, shading: { fill: "FFF2CC", type: ShadingType.CLEAR }, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "合  计", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, shading: { fill: "FFF2CC", type: ShadingType.CLEAR }, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "￥__________", bold: true, size: 21, font: "SimSun" })] })] }),
                        new TableCell({ borders, shading: { fill: "FFF2CC", type: ShadingType.CLEAR }, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ children: [new TextRun({ text: "（不含税）", bold: true, size: 19, font: "SimSun" })] })] }),
                        new TableCell({ borders, shading: { fill: "FFF2CC", type: ShadingType.CLEAR }, margins: { top: 60, bottom: 60, left: 100, right: 100 },
                            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [new TextRun({ text: "—", size: 19, font: "SimSun" })] })] })
                    ]})
                ]
            }),
            createEmptyLine(),

            createHeading2("4.2 付款方式"),
            createParagraph("甲方应将款项汇入乙方指定账户：", { indent: true }),
            createEmptyLine(),
            createInfoRow("户名", "（由乙方提供）"),
            createInfoRow("开户银行", "（由乙方提供）"),
            createInfoRow("银行账号", "（由乙方提供）"),
            createInfoRow("统一社会信用代码", "（由乙方提供）"),
            createEmptyLine(),

            // ==================== 第五条：交付与验收 ====================
            createHeading1("第五条  交付与验收"),

            createHeading2("5.1 交付时间"),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: "标准版交付：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "甲方完成首期付款后____个工作日内，乙方通过加密方式交付全部标准版交付物。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: "定制开发交付：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "双方确认《定制需求说明书》后____个工作日内完成开发和初步验收。复杂定制项目以附件约定工期为准。", size: 22, font: "SimSun" })
                ]
            }),
            createEmptyLine(),

            createHeading2("5.2 验收标准"),
            createParagraph("甲方应在收到交付物后____个工作日内完成验收。验收合格应符合以下条件：", { indent: true }),
            createEmptyLine(),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "软件能够正常安装部署于甲方提供的部署环境中；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "第二条所列之标准版功能模块均可正常运行；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "API 接口响应符合 OpenAI 兼容规范，流式输出正常；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "计费引擎 Token 统计误差不超过±2%；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "管理后台各项操作无功能性缺陷（UI 微调类问题除外）；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "定制功能（如有）达到《定制需求说明书》约定的验收标准。", size: 22, font: "SimSun" })]
            }),
            createEmptyLine(),

            createHeading2("5.3 验收流程"),
            createParagraph("（1）乙方提交交付物及《验收确认单》；", { indent: true }),
            createParagraph("（2）甲方组织内部测试，记录发现的问题；", { indent: true }),
            createParagraph("（3）双方确认问题清单，乙方在约定时限内修复；", { indent: true }),
            createParagraph("（4）复测通过后，双方签署《验收确认单》，视为验收合格。", { indent: true }),
            createEmptyLine(),

            // ==================== 第六条：知识产权 ====================
            createHeading1("第六条  知识产权"),

            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "原有知识产权归属：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "TraceHub 标准交付版的原有代码、文档、设计等知识产权归乙方所有。本项目采用 MIT 开源协议发布，甲方在遵守 MIT 协议的前提下，享有使用、复制、修改、合并、发布、分发、再授权和/或销售软件副本的权利。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "定制开发成果归属：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "基于本合同产生的定制开发成果（新增功能模块、定制代码、适配器等），其知识产权归甲方所有。乙方不得将该等定制成果用于其他客户或产品，未经甲方书面许可不得开源发布。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "第三方组件：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "软件中使用的第三方开源组件（Go 依赖库、前端框架、字体等）遵循各自原有的开源协议（如 Apache-2.0、MIT、BSD 等），乙方不对此部分主张权利，也不承担因第三方组件引起的知识产权纠纷责任。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "品牌标识：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: `甲方有权在使用过程中替换站点 Logo、名称、配色等品牌元素。"TraceHub"、"迹路径"、"循迹未来"等商标/品牌名称的使用需经乙方书面授权。`, size: 22, font: "SimSun" })
                ]
            }),
            createEmptyLine(),

            // ==================== 第七条：保密条款 ====================
            createHeading1("第七条  保密条款"),
            createParagraph("双方应对在合作过程中获悉的对方商业秘密和技术信息严格保密，未经对方书面同意不得向任何第三方披露。保密义务在本合同终止后____年内持续有效。保密信息范围包括但不限于：", { indent: true }),
            createEmptyLine(),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "软件源代码、设计文档、架构图纸等技术资料；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "甲方的业务数据、用户信息、财务数据；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "双方的商业模式、定价策略、客户名单；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "本合同的条款内容与交易金额；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "法律法规规定的其他应当保密的信息。", size: 22, font: "SimSun" })]
            }),
            createEmptyLine(),

            // ==================== 第八条：售后服务与技术支持 ====================
            createHeading1("第八条  售后服务与技术支持"),

            createHeading2("8.1 免费质保期"),
            createParagraph("自验收合格之日起，乙方提供____天的免费质保服务。质保期内，对于标准版功能范围内的 Bug 和缺陷，乙方承诺：", { indent: true }),
            createEmptyLine(),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: "致命性问题（导致服务不可用）：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "____小时内响应，____小时内修复；", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: "严重性问题（主要功能异常）：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "____小时内响应，____个工作日内修复；", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [
                    new TextRun({ text: "一般性问题（非核心功能或体验优化）：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "____个工作日内响应，纳入后续版本迭代计划。", size: 22, font: "SimSun" })
                ]
            }),
            createEmptyLine(),

            createHeading2("8.2 有偿维保服务"),
            createParagraph("免费质保期满后，甲方可选择购买年度维保服务，服务内容包括：", { indent: true }),
            createEmptyLine(),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "标准版功能 Bug 修复与安全漏洞补丁推送；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "新版本的升级通知与升级指导（不含定制功能的兼容性适配）；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "远程技术支持（工作日 9:00-18:00）；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "每季度一次健康检查报告（需提供必要访问权限）。", size: 22, font: "SimSun" })]
            }),
            createEmptyLine(),

            createHeading2("8.3 不在服务范围内的事项"),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "甲方自行修改源代码导致的问题；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "第三方上游 API 服务商（如 OpenAI、Anthropic 等）的服务中断或变更；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "甲方部署环境的硬件故障、网络故障或配置不当；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "超出标准版范围的新增需求或定制开发；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "不可抗力因素造成的影响。", size: 22, font: "SimSun" })]
            }),
            createEmptyLine(),

            // ==================== 第九条：违约责任 ====================
            createHeading1("第九条  违约责任"),

            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "甲方逾期付款：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "每逾期一日，应按应付未付金额的____‰向乙方支付违约金。逾期超过____日的，乙方有权暂停服务或解除合同，且不退还已收取的款项。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "乙方逾期交付：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "非因甲方原因导致逾期交付的，每逾期一日，乙方应按合同总金额的____‰向甲方支付违约金。逾期超过____日的，甲方有权解除合同并要求全额退款。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "质量不符合约定：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "若交付成果经两次修复仍无法通过验收的，甲方有权要求乙方继续整改或解除合同并退还未通过部分的相应款项。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "知识产权侵权：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "因乙方原因导致交付成果侵犯第三方知识产权的，由乙方承担全部法律责任并赔偿甲方因此遭受的全部损失。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "保密违约：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "任何一方违反保密义务的，应向对方支付违约金人民币________元；造成实际损失超过违约金的，还应赔偿差额部分。", size: 22, font: "SimSun" })
                ]
            }),
            createEmptyLine(),

            // ==================== 第十条：免责条款与不可抗力 ====================
            createHeading1("第十条  免责条款与不可抗力"),
            createParagraph("因以下原因导致合同不能履行或不能完全履行的，遭受不可抗力的一方不承担违约责任，但应及时通知对方并在____日内提供有效证明：", { indent: true }),
            createEmptyLine(),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "自然灾害（地震、洪水、台风等）；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "政府行为或法律法规政策变化；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "战争、罢工、骚乱等社会异常事件；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "互联网基础设施大面积故障（骨干网、电力等）；", size: 22, font: "SimSun" })]
            }),
            new Paragraph({
                numbering: { reference: "bullets", level: 0 },
                spacing: { after: 80, line: 360 },
                children: [new TextRun({ text: "上游 AI 服务商（OpenAI/Anthropic 等）官方 API 服务中断或策略变更。", size: 22, font: "SimSun" })]
            }),
            createEmptyLine(),

            // ==================== 第十一条：争议解决 ====================
            createHeading1("第十一条  争议解决"),
            createParagraph("因本合同引起的或与本合同有关的任何争议，双方应首先通过友好协商解决。协商不成的，任何一方均有权向________________人民法院提起诉讼。", { indent: true }),
            createEmptyLine(),

            // ==================== 第十二条：其他约定 ====================
            createHeading1("第十二条  其他约定"),

            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "通知送达：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "与本合同有关的通知应以书面形式发出。甲方联系地址以合同首部载明地址为准，乙方联系地址以营业执照登记地址为准。一方变更通讯地址的，应提前____日书面通知对方。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "合同变更与补充：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "对本合同的任何修改、补充均须经双方书面确认，并以书面形式（包括电子数据交换）作出。口头承诺无效。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "合同效力：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "本合同一式____份，甲乙双方各执____份，具有同等法律效力。自双方法定代表人或授权代表签字并加盖公章（或合同专用章）之日起生效。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "附件效力：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "本合同附件是合同不可分割的组成部分，与合同正文具有同等法律效力。附件包括：（1）《定制需求说明书》（如有）；（2）《验收确认单》（模板）；（3）其他双方书面确认的技术规格文件。", size: 22, font: "SimSun" })
                ]
            }),
            new Paragraph({
                numbering: { reference: "sub-numbers", level: 0 },
                spacing: { after: 100, line: 380 },
                children: [
                    new TextRun({ text: "完整性声明：", bold: true, size: 22, font: "SimSun" }),
                    new TextRun({ text: "本合同构成双方就本合同标的达成的完整合意，取代此前就同一事项的所有口头或书面沟通、谅解或协议。", size: 22, font: "SimSun" })
                ]
            }),
            createEmptyLine(),

            // ==================== 签字页 ====================
            new Paragraph({ children: [new PageBreak()] }),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                spacing: { after: 400 },
                children: [new TextRun({ text: "【签 字 页】", bold: true, size: 32, font: "SimHei" })]
            }),
            createEmptyLine(),
            createEmptyLine(),

            // 甲方签章区
            new Paragraph({
                spacing: { after: 200 },
                children: [new TextRun({ text: "甲方（委托方）：", bold: true, size: 24, font: "SimSun" })]
            }),
            createInfoRow("单位全称", ""),
            createInfoRow("统一社会信用代码", ""),
            createInfoRow("法定代表人/负责人", ""),
            createInfoRow("授权代表（签字）", ""),
            createInfoRow("联系电话", ""),
            createInfoRow("电子邮箱", ""),
            createInfoRow("通讯地址", ""),
            createEmptyLine(),
            new Paragraph({
                spacing: { after: 100 },
                children: [
                    new TextRun({ text: "（加盖公章/合同专用章）", size: 22, font: "SimSun" }),
                    new TextRun({ text: "                                              ", size: 22 }),
                    new TextRun({ text: "    签署日期：____年____月____日", size: 22, font: "SimSun" })
                ]
            }),
            createEmptyLine(), createEmptyLine(),

            // 乙方签章区
            new Paragraph({
                spacing: { after: 200 },
                children: [new TextRun({ text: "乙方（开发方）：循迹未来团队", bold: true, size: 24, font: "SimSun" })]
            }),
            createInfoRow("团队/公司名称", ""),
            createInfoRow("统一社会信用代码", ""),
            createInfoRow("负责人/法定代表人", ""),
            createInfoRow("授权代表（签字）", ""),
            createInfoRow("联系电话", ""),
            createInfoRow("电子邮箱", ""),
            createInfoRow("通讯地址", ""),
            createEmptyLine(),
            new Paragraph({
                spacing: { after: 100 },
                children: [
                    new TextRun({ text: "（加盖公章/合同专用章）", size: 22, font: "SimSun" }),
                    new TextRun({ text: "                                              ", size: 22 }),
                    new TextRun({ text: "    签署日期：____年____月____日", size: 22, font: "SimSun" })
                ]
            }),
            createEmptyLine(), createEmptyLine(), createEmptyLine(),
            
            // 合同结尾提示
            new Paragraph({
                border: { bottom: { style: BorderStyle.SINGLE, size: 6, color: "2E75B6", space: 1 } },
                spacing: { before: 400 },
                children: [new TextRun({ text: " ", size: 12 })]
            }),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                spacing: { before: 100 },
                children: [new TextRun({ text: "— 本合同完 —", size: 20, font: "SimSun", color: "808080" })]
            })
        ]
    }]
});

// 生成文件
Packer.toBuffer(doc).then(buffer => {
    const outputPath = 'd:/开源项目/TraceHub/docs/TraceHub-软件产品销售及定制开发服务合同.docx';
    fs.writeFileSync(outputPath, buffer);
    console.log(`合同文档已生成：${outputPath}`);
}).catch(err => console.error('生成失败:', err));
