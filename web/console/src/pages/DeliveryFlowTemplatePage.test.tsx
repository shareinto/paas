import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { DeliveryFlowTemplatePage } from './DeliveryFlowTemplatePage';

function renderPage() {
  return render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <DeliveryFlowTemplatePage />
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );
}

afterEach(() => cleanup());

test('租户交付流模板展示 Stage 卡片并使用弹窗编辑', async () => {
  renderPage();

  expect(await screen.findByRole('heading', { name: '租户交付流模板' })).toBeInTheDocument();
  const devCard = await screen.findByLabelText('dev Stage 模板');
  expect(within(devCard).getByText('开发')).toBeInTheDocument();
  expect(devCard.querySelector('.stage-color-strip')).toBeTruthy();
  expect(within(devCard).getByRole('button', { name: '绑定集群' })).toBeInTheDocument();
  expect(within(devCard).getByRole('button', { name: '编辑' })).toBeInTheDocument();
  expect(within(devCard).getByRole('button', { name: '删除' })).toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: '添加 Stage' }));
  const stageDialog = await screen.findByRole('dialog', { name: '添加 Stage' });
  expect(within(stageDialog).getByLabelText('Stage key')).toBeInTheDocument();
  expect(within(stageDialog).getByLabelText('Stage 颜色')).toBeInTheDocument();
  expect(within(stageDialog).getByText('禁止物理删除或修改 Stage key')).toBeInTheDocument();
});

test('Stage 集群绑定使用弹窗和多选列表', async () => {
  renderPage();

  const devCard = await screen.findByLabelText('dev Stage 模板');
  await userEvent.click(within(devCard).getByRole('button', { name: '绑定集群' }));

  const dialog = await screen.findByRole('dialog', { name: '绑定集群' });
  expect(within(dialog).getByText('绑定到租户级 Stage，保存后进入该 Stage 的可选集群池。')).toBeInTheDocument();
  expect(within(dialog).getByText('同一集群可绑定多个 Stage，绑定变更仅影响后续发布。')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('可选集群')).toBeInTheDocument();
  expect(screen.queryByText('Stage 绑定')).not.toBeInTheDocument();
});
