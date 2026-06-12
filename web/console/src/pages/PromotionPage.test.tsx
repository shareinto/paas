import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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

  expect(await screen.findByLabelText('应用部署 DAG')).toBeInTheDocument();
  expect(screen.getByText('拖拽 Freight 到目标 Stage，系统按交付流 DAG 校验上游依赖、审批和验证要求。')).toBeInTheDocument();
  const timeline = await screen.findByLabelText('Freight 时间轴');
  const cards = within(timeline).getAllByTestId('freight-card');

  expect(cards.map((card) => within(card).getByTestId('freight-name').textContent)).toEqual([
    '20260609.1',
    '20260610.1',
    '20260611.1'
  ]);
});

test('Stage 卡片显示 DAG 投影和部署状态', async () => {
  renderPromotionPage();

  for (const stage of ['dev', 'test', 'staging', 'prod']) {
    const card = await screen.findByLabelText(`${stage} Stage`);
    expect(within(card).queryByRole('button', { name: '发布' })).not.toBeInTheDocument();
    expect(within(card).getByRole('button', { name: '验证' })).toBeInTheDocument();
    expect(within(card).getByText('绑定集群')).toBeInTheDocument();
    expect(within(card).getByText('当前 Freight')).toBeInTheDocument();
    expect(within(card).getByText('上游 Stage')).toBeInTheDocument();
    expect(within(card).getByText('验证状态')).toBeInTheDocument();
  }
  expect(within(await screen.findByLabelText('prod Stage')).getByText('需审批')).toBeInTheDocument();
});

test('拖拽不可发布 Freight 到 dev 后提示不可发布', async () => {
  renderPromotionPage();

  const devCard = await screen.findByLabelText('dev Stage');
  dragFreightToStage(await freightCardByName('20260610.1'), devCard);

  expect(await screen.findByText('该 Freight 当前不能发布到目标 Stage')).toBeInTheDocument();
  expect(within(devCard).queryByLabelText('dev 发布确认')).not.toBeInTheDocument();
});

test('拖拽 Freight 到 Stage 后在卡片内确认发布并展示 Workload 镜像', async () => {
  renderPromotionPage();

  const devCard = await screen.findByLabelText('dev Stage');
  dragFreightToStage(await freightCardByName('20260609.1'), devCard);

  const confirm = await within(devCard).findByLabelText('dev 发布确认');
  expect(screen.queryByTestId('promotion-confirm-panel')).not.toBeInTheDocument();
  expect(within(confirm).getByText('20260609.1')).toBeInTheDocument();
  expect(within(confirm).getByText('发布到 上海集群')).toBeInTheDocument();
  expect(screen.queryByRole('dialog', { name: '发布确认' })).not.toBeInTheDocument();
});

test('确认发布后更新 Stage 当前 Freight', async () => {
  renderPromotionPage();

  const devCard = await screen.findByLabelText('dev Stage');
  dragFreightToStage(await freightCardByName('20260609.1'), devCard);
  const confirm = await within(devCard).findByLabelText('dev 发布确认');
  await userEvent.click(within(confirm).getByRole('button', { name: '确认发布' }));

  await waitFor(() => {
    expect(within(devCard).getByText('20260609.1')).toBeInTheDocument();
  });
  expect(await screen.findByText(/20260609\.1 已提交到 dev/)).toBeInTheDocument();
  expect(screen.queryByRole('dialog', { name: '发布确认' })).not.toBeInTheDocument();
});

test('prod Stage 显示审批标签且不提供发布按钮', async () => {
  renderPromotionPage();

  const prodCard = await screen.findByLabelText('prod Stage');
  expect(within(prodCard).getByText('需审批')).toBeInTheDocument();
  expect(within(prodCard).queryByRole('button', { name: '发布' })).not.toBeInTheDocument();
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

async function freightCardByName(name: string) {
  const timeline = await screen.findByLabelText('Freight 时间轴');
  return within(timeline).getAllByTestId('freight-card').find((card) => within(card).queryByText(name)) as HTMLElement;
}

function dragFreightToStage(freightCard: HTMLElement, stageCard: HTMLElement) {
  const data = new Map<string, string>();
  const dataTransfer = {
    setData: (type: string, value: string) => data.set(type, value),
    getData: (type: string) => data.get(type) || ''
  } as DataTransfer;
  fireEvent.dragStart(freightCard, { dataTransfer });
  fireEvent.dragOver(stageCard, { dataTransfer });
  fireEvent.drop(stageCard, { dataTransfer });
}
