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

test('Freight 按创建时间从左到右显示', async () => {
  renderPromotionPage();

  const timeline = await screen.findByLabelText('Freight 时间轴');
  const cards = within(timeline).getAllByTestId('freight-card');

  expect(cards.map((card) => within(card).getByTestId('freight-name').textContent)).toEqual([
    '20260609.1',
    '20260610.1',
    '20260611.1'
  ]);
});

test('Stage 卡片显示发布按钮', async () => {
  renderPromotionPage();

  for (const stage of ['dev', 'test', 'staging', 'prod']) {
    const card = await screen.findByLabelText(`${stage} Stage`);
    expect(within(card).getByRole('button', { name: '发布' })).toBeInTheDocument();
  }
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

test('选择 Freight 后确认弹窗显示所有 Workload 镜像', async () => {
  renderPromotionPage();

  const devCard = await screen.findByLabelText('dev Stage');
  await userEvent.click(within(devCard).getByRole('button', { name: '发布' }));
  await userEvent.click(await screen.findByRole('button', { name: /选择 Freight 20260609\.1/ }));

  const dialog = await screen.findByRole('dialog', { name: '确认发布' });
  expect(within(dialog).getByText('前端入口')).toBeInTheDocument();
  expect(within(dialog).getByText('registry.local/order-frontend:20260609.1')).toBeInTheDocument();
  expect(within(dialog).getByText('订单接口')).toBeInTheDocument();
  expect(within(dialog).getByText('registry.local/order-api:20260609.1')).toBeInTheDocument();
  expect(within(dialog).getByText('异步任务')).toBeInTheDocument();
  expect(within(dialog).getByText('registry.local/order-worker:20260609.1')).toBeInTheDocument();
});

test('prod 发布显示审批提示和禁止自审批提示', async () => {
  renderPromotionPage();

  const prodCard = await screen.findByLabelText('prod Stage');
  await userEvent.click(within(prodCard).getByRole('button', { name: '发布' }));
  await userEvent.click(await screen.findByRole('button', { name: /选择 Freight 20260611\.1/ }));

  const dialog = await screen.findByRole('dialog', { name: '确认发布' });
  expect(within(dialog).getByText('审批人数：至少 2 人')).toBeInTheDocument();
  expect(within(dialog).getByText('审批人范围：生产审批人')).toBeInTheDocument();
  expect(within(dialog).getByText('禁止发起人自审批')).toBeInTheDocument();
});

test('创建 Freight 抽屉列出所有启用 Workload，未覆盖全部 Workload 时提交按钮禁用', async () => {
  renderPromotionPage();

  await userEvent.click(await screen.findByRole('button', { name: '创建 Freight' }));

  const drawer = await screen.findByRole('dialog', { name: '创建 Freight' });
  expect(within(drawer).getByText('前端入口')).toBeInTheDocument();
  expect(within(drawer).getByText('订单接口')).toBeInTheDocument();
  expect(within(drawer).getByText('异步任务')).toBeInTheDocument();
  expect(within(drawer).getByRole('button', { name: '创建' })).toBeDisabled();

  await userEvent.click(within(drawer).getByLabelText('前端入口流水线产物'));
  await userEvent.click(await screen.findByTitle('registry.local/order-frontend:20260611.1'));
  await userEvent.click(within(drawer).getByLabelText('订单接口流水线产物'));
  await userEvent.click(await screen.findByTitle('registry.local/order-api:20260611.1'));

  expect(within(drawer).getByRole('button', { name: '创建' })).toBeDisabled();
});

test('自定义镜像 tag 显示风险提示', async () => {
  renderPromotionPage();

  await userEvent.click(await screen.findByRole('button', { name: '创建 Freight' }));
  const drawer = await screen.findByRole('dialog', { name: '创建 Freight' });

  await userEvent.click(within(drawer).getByLabelText('异步任务自定义镜像'));
  const imageInput = within(drawer).getByPlaceholderText('请输入完整镜像地址');
  await userEvent.type(imageInput, 'registry.local/order-worker:latest');

  await waitFor(() => {
    expect(within(drawer).getByText('镜像 tag 可能被覆盖，建议使用 digest。')).toBeInTheDocument();
  });
});
