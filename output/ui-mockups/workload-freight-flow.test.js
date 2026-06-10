const fs = require('fs');
const path = require('path');
const assert = require('assert');

const html = fs.readFileSync(path.join(__dirname, 'workload-freight-flow.html'), 'utf8');

function count(pattern) {
  return (html.match(pattern) || []).length;
}

assert(!html.includes('data-screen="workload-create"'), '创建 Workload 不应作为 Tab/独立页面存在');
assert(!html.includes('data-screen="deploy-config"'), '部署配置不应作为 Tab/独立页面存在');
assert(!html.includes('data-screen="freight-create"'), '创建 Freight 不应作为 Tab/独立页面存在');
assert(!html.includes('data-screen="freight-detail"'), 'Freight 详情不应作为 Tab/独立页面存在');
assert.strictEqual(count(/class="tab(?: active)?" data-screen="/g), 2, '顶部应只保留两个 Tab');
assert(html.includes('id="workloadModal"'), '应提供创建 Workload 弹窗');
assert(html.includes('id="deployDrawer"'), '应提供部署配置抽屉');
assert(html.includes('id="freightDrawer"'), '应提供创建 Freight 抽屉');
assert(html.includes('class="freight-rail"'), '发布晋级页应包含 Freight 时间轴');
assert(html.includes('data-stage="dev"'), 'Stage 发布按钮应声明目标 Stage');
assert(html.includes('selectable'), '点击 Stage 发布后应点亮可发布 Freight');
assert(html.includes('confirmPromotion'), '选择 Freight 后应显示发布确认操作');

console.log('workload-freight-flow static prototype checks passed');
