const fs = require('fs');
const path = require('path');
const assert = require('assert');

const html = fs.readFileSync(path.join(__dirname, 'stage-delivery-flow.html'), 'utf8');

function count(pattern) {
  return (html.match(pattern) || []).length;
}

assert.strictEqual(count(/class="tab(?: active)?" data-screen="/g), 2, '应提供两个主视图');
assert(html.includes('data-screen="template"'), '应包含租户交付流模板视图');
assert(!html.includes('data-screen="bindings"'), '不应保留独立 Stage 集群绑定 Tab');
assert(!html.includes('id="bindings"'), '不应保留独立 Stage 集群绑定页面');
assert(html.includes('data-screen="release"'), '应包含应用 Stage 发布视图');
assert(!html.includes('data-screen="verify"'), '不应保留独立审批与验证 Tab');
assert(!html.includes('id="verify"'), '不应保留独立审批与验证页面');

assert(html.includes('data-stage-key="dev"'), '应使用稳定 Stage key 表达 dev');
assert(html.includes('data-stage-key="prod"'), '应使用稳定 Stage key 表达 prod');
assert(html.includes('verify_roles'), '模板应包含 verify 角色策略');
assert(html.includes('approve_roles'), '模板应包含 approve 角色策略');
assert(html.includes('禁止物理删除或修改 Stage key'), '应提示 Stage key 稳定不可改');
assert(html.includes('id="stageTemplateModal"'), '添加和编辑 Stage 模板应使用弹窗');
assert(html.includes('onclick="openStageTemplateModal()'), '模板页右上角应提供添加按钮');
assert(html.includes('onclick="openStageTemplateModal('), 'Stage 卡片应提供编辑按钮并打开弹窗');
assert(html.includes('onclick="deleteStageTemplate('), 'Stage 卡片应提供删除按钮');
assert(html.includes('onclick="openClusterBindingModal('), 'Stage 卡片应提供绑定集群按钮');
assert(!html.includes('<aside class="panel">\n              <div class="panel-header"><h2>编辑 Stage 策略</h2>'), '模板编辑不应使用右侧抽屉或侧栏面板');
assert(html.includes('class="stage-color-strip"'), 'Stage 卡片顶部应有颜色条');
assert(html.includes('data-stage-color='), 'Stage 卡片应声明模板颜色');
assert(html.includes('id="stageTemplateColor"'), 'Stage 模板弹窗应允许设置颜色');
assert(html.includes('type="color"'), 'Stage 颜色应使用颜色选择控件');
assert(html.includes('.stage-color-strip { min-height: 48px;'), 'Stage 顶部色条纵向高度应增加');
assert(html.includes('class="stage-strip-title"'), 'Stage 名称应显示在顶部色条内');
assert(html.includes('.stage-strip-title { color: #fff;'), 'Stage 色条内名称应使用白色字体');

assert(!html.includes('id="bindingDrawer"'), 'Stage 集群绑定不应使用右侧抽屉');
assert(html.includes('id="clusterBindingModal"'), 'Stage 集群绑定应使用弹窗');
assert(html.includes('data-cluster="cluster-shanghai"'), '应展示可绑定集群');
assert(html.includes('name="stageBindingClusters"'), '绑定集群弹窗应提供集群多选列表');
assert(html.includes('同一集群可绑定多个 Stage'), '应表达集群和 Stage 多对多关系');
assert(html.includes('仅影响后续发布'), '绑定变更应提示只影响后续发布');

assert(html.includes('id="promotionModal"'), '应提供发布确认弹窗');
assert(html.includes('name="targetClusters"'), '发布时应选择目标集群子集');
assert(html.includes('id="namespaceOverride"'), '发布时应允许覆盖 namespace');
assert(html.includes('默认使用项目名称'), 'namespace 默认策略应可见');

assert(!html.includes('id="verifyDrawer"'), '人工验证不应使用右侧抽屉');
assert(html.includes('id="verifyModal"'), '人工验证应使用弹窗');
assert(html.includes('id="approvalModal"'), '审批应使用弹窗');
assert(html.includes('onclick="openVerifyModal('), '发布页 Stage 卡片应提供验证按钮');
assert(html.includes('onclick="openApprovalModal('), 'Freight 卡片应提供审批按钮');
assert(html.includes('验证通过'), '应提供验证通过操作');
assert(html.includes('验证不通过'), '应提供验证不通过操作');
assert(html.includes('审批通过'), '应提供审批通过操作');
assert(html.includes('审批拒绝'), '应提供审批拒绝操作');
assert(html.includes('只要求已有部署记录'), '应说明 verify 不强制依赖健康状态');
assert(html.includes('按最新模板规则校验'), '应提示进行中发布按最新模板执行');

assert(html.includes('function openPromotion'), '应包含发布交互脚本');
assert(html.includes('function completeVerify'), '应包含验证交互脚本');
assert(html.includes('function openVerifyModal'), '应包含验证弹窗脚本');
assert(html.includes('function openApprovalModal'), '应包含审批弹窗脚本');
assert(html.includes('function completeApproval'), '应包含审批处理脚本');
assert(html.includes('function openClusterBindingModal'), '应包含绑定集群弹窗脚本');
assert(html.includes('function saveClusterBinding'), '应包含保存 Stage 集群绑定脚本');
assert(html.includes('function openStageTemplateModal'), '应包含添加/编辑 Stage 模板弹窗脚本');
assert(html.includes('function saveStageTemplate'), '应包含保存 Stage 模板脚本');
assert(html.includes('function deleteStageTemplate'), '应包含删除 Stage 模板脚本');
assert(html.includes('stageTemplateColor'), '弹窗脚本应处理 Stage 颜色');

console.log('stage-delivery-flow static prototype checks passed');
