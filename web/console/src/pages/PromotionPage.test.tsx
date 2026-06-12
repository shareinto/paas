import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { PromotionPage } from './PromotionPage';

function renderPromotionPage() {
  return render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/promotions']}>
          <PromotionPage />
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );
}

afterEach(() => {
  cleanup();
});

test('Freight 按创建时间从左到右显示并展示流程引导', async () => {
  renderPromotionPage();

  const flow = screen.getByLabelText('交付流程');
  ['创建 Workload', '配置环境差异', '创建完整 Freight', '选择目标 Stage', '发布晋级', '回滚历史 Freight'].forEach((name) => {
    expect(within(flow).getByText(name)).toBeInTheDocument();
  });
  const timeline = await screen.findByLabelText('Freight 时间轴');
  const cards = within(timeline).getAllByTestId('freight-card');

  expect(cards.map((card) => within(card).getByTestId('freight-name').textContent)).toEqual([
    '20260609.1',
    '20260610.1',
    '20260611.1'
  ]);
});

test('Stage 卡片显示发布按钮和环境摘要', async () => {
  renderPromotionPage();

  for (const stage of ['dev', 'test', 'staging', 'prod']) {
    const card = await screen.findByLabelText(`${stage} Stage`);
    expect(within(card).getByRole('button', { name: '发布' })).toBeInTheDocument();
    expect(within(card).getByRole('button', { name: '验证' })).toBeInTheDocument();
    expect(within(card).getByText('集群池')).toBeInTheDocument();
    expect(within(card).getByText('当前 Freight')).toBeInTheDocument();
    expect(within(card).getByText('副本')).toBeInTheDocument();
    expect(within(card).getByText('域名')).toBeInTheDocument();
    expect(within(card).getByText('配置')).toBeInTheDocument();
  }
  expect(within(await screen.findByLabelText('prod Stage')).getByText('生产需审批')).toBeInTheDocument();
});

test('点击 dev 发布后只点亮 dev 可发布 Freight，其他 Freight 被置灰禁用', async () => {
  renderPromotionPage();

  const devCard = await screen.findByLabelText('dev Stage');
  await userEvent.click(within(devCard).getByRole('button', { name: '发布' }));

  const freightA = await screen.findByRole('button', { name: /选择 Freight 20260609\.1/ });
  const freightB = screen.getByRole('button', { name: /选择 Freight 20260610\.1/ });
  const freightC = screen.getByRole('button', { name: /选择 Freight 20260611\.1/ });

  expect(freightA).toHaveAttribute('data-eligible', 'true');
  expect(freightB).toBeDisabled();
  expect(freightB).toHaveAttribute('data-eligible', 'false');
  expect(freightC).toBeDisabled();
  expect(freightC).toHaveAttribute('data-eligible', 'false');
});

test('发布确认使用弹窗选择 Freight、目标集群子集和 Namespace', async () => {
  renderPromotionPage();

  const devCard = await screen.findByLabelText('dev Stage');
  await userEvent.click(within(devCard).getByRole('button', { name: '发布' }));

  const dialog = await screen.findByRole('dialog', { name: '发布确认' });
  expect(screen.queryByTestId('promotion-confirm-panel')).not.toBeInTheDocument();
  expect(within(dialog).getByText('目标 Stage')).toBeInTheDocument();
  expect(within(dialog).getByText('dev')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('选择 Freight')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('目标集群')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('Namespace')).toHaveValue('订单平台');

  await userEvent.click(within(dialog).getByRole('radio', { name: /20260609\.1/ }));
  expect(within(dialog).getByText('前端入口')).toBeInTheDocument();
  expect(within(dialog).getByText('registry.local/order-frontend:20260609.1')).toBeInTheDocument();
  expect(within(dialog).getByText('订单接口')).toBeInTheDocument();
  expect(within(dialog).getByText('registry.local/order-api:20260609.1')).toBeInTheDocument();
  expect(within(dialog).getByText('异步任务')).toBeInTheDocument();
  expect(within(dialog).getByText('registry.local/order-worker:20260609.1')).toBeInTheDocument();
});

test('确认发布后更新 Stage 当前 Freight', async () => {
  renderPromotionPage();

  const devCard = await screen.findByLabelText('dev Stage');
  await userEvent.click(within(devCard).getByRole('button', { name: '发布' }));
  const dialog = await screen.findByRole('dialog', { name: '发布确认' });
  await userEvent.click(within(dialog).getByRole('radio', { name: /20260609\.1/ }));
  await userEvent.click(within(dialog).getByRole('button', { name: '确认发布' }));

  await waitFor(() => {
    expect(within(devCard).getByText('20260609.1')).toBeInTheDocument();
  });
  expect(await screen.findByText(/20260609\.1 已提交到 dev/)).toBeInTheDocument();
});

test('prod 发布在弹窗显示审批提示和禁止自审批提示', async () => {
  renderPromotionPage();

  const prodCard = await screen.findByLabelText('prod Stage');
  await userEvent.click(within(prodCard).getByRole('button', { name: '发布' }));

  const dialog = await screen.findByRole('dialog', { name: '发布确认' });
  await userEvent.click(within(dialog).getByRole('radio', { name: /20260611\.1/ }));
  expect(within(dialog).getByText('审批人数：至少 2 人')).toBeInTheDocument();
  expect(within(dialog).getByText('审批人范围：生产审批人')).toBeInTheDocument();
  expect(within(dialog).getByText('禁止发起人自审批')).toBeInTheDocument();
});

test('Freight 卡片审批按钮打开 Freight 审批弹窗', async () => {
  renderPromotionPage();

  const timeline = await screen.findByLabelText('Freight 时间轴');
  const freightCard = within(timeline).getAllByTestId('freight-card')[0];
  await userEvent.click(within(freightCard).getByRole('button', { name: '审批' }));

  const dialog = await screen.findByRole('dialog', { name: 'Freight 审批' });
  expect(within(dialog).getByText('审批 Freight')).toBeInTheDocument();
  expect(within(dialog).getByText('20260609.1')).toBeInTheDocument();
  expect(within(dialog).getByRole('combobox', { name: '目标 Stage' })).toBeInTheDocument();
  expect(within(dialog).getByLabelText('审批意见')).toBeInTheDocument();
  expect(within(dialog).getByRole('button', { name: '审批拒绝' })).toBeInTheDocument();
  expect(within(dialog).getByRole('button', { name: '审批通过' })).toBeInTheDocument();
});

test('Stage 验证按钮打开人工验证弹窗并展示部署证据', async () => {
  renderPromotionPage();

  const devCard = await screen.findByLabelText('dev Stage');
  await userEvent.click(within(devCard).getByRole('button', { name: '验证' }));

  const dialog = await screen.findByRole('dialog', { name: '人工验证' });
  expect(within(dialog).getByText('验证 Stage')).toBeInTheDocument();
  expect(within(dialog).getByText('dev')).toBeInTheDocument();
  expect(within(dialog).getByText('Argo CD 同步')).toBeInTheDocument();
  expect(within(dialog).getByText('健康状态')).toBeInTheDocument();
  expect(within(dialog).getByText('Agent 状态')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('验证备注')).toBeInTheDocument();
  expect(within(dialog).getByRole('button', { name: '验证不通过' })).toBeInTheDocument();
  expect(within(dialog).getByRole('button', { name: '验证通过' })).toBeInTheDocument();
});

test('创建 Freight 抽屉使用表格列出所有启用 Workload 并按行校验', async () => {
  renderPromotionPage();

  await userEvent.click(await screen.findByRole('button', { name: '创建 Freight' }));

  const drawer = await screen.findByRole('dialog', { name: '创建 Freight' });
  ['Workload', '镜像来源', '流水线产物', '自定义镜像', '校验', '说明'].forEach((name) => {
    expect(within(drawer).getByRole('columnheader', { name })).toBeInTheDocument();
  });
  expect(within(drawer).getByText('前端入口')).toBeInTheDocument();
  expect(within(drawer).getByText('订单接口')).toBeInTheDocument();
  expect(within(drawer).getByText('异步任务')).toBeInTheDocument();
  expect(within(drawer).getByRole('button', { name: '创建 Freight' })).toBeDisabled();

  await userEvent.click(within(drawer).getByRole('button', { name: '从最新成功版本填充' }));
  expect(within(drawer).getByText('已覆盖全部 Workload')).toBeInTheDocument();
  expect(within(drawer).getByRole('button', { name: '创建 Freight' })).not.toBeDisabled();
});

test('创建 Freight 支持从历史 Freight 复制和自定义镜像 tag 风险提示', async () => {
  renderPromotionPage();

  await userEvent.click(await screen.findByRole('button', { name: '创建 Freight' }));
  const drawer = await screen.findByRole('dialog', { name: '创建 Freight' });

  await userEvent.click(within(drawer).getByRole('button', { name: '从历史 Freight 复制' }));
  expect(within(drawer).getByText('已覆盖全部 Workload')).toBeInTheDocument();

  const workerRow = (within(drawer).getByText('异步任务').closest('tr') || drawer) as HTMLElement;
  await userEvent.click(within(workerRow).getByRole('combobox', { name: '异步任务镜像来源' }));
  await userEvent.click(await screen.findByTitle('自定义镜像'));
  const imageInput = within(workerRow).getByLabelText('异步任务自定义镜像');
  await userEvent.clear(imageInput);
  await userEvent.type(imageInput, 'registry.local/order-worker:latest');

  await waitFor(() => {
    expect(within(drawer).getByText('镜像 tag 可能被覆盖，建议使用 digest。')).toBeInTheDocument();
  });
});
