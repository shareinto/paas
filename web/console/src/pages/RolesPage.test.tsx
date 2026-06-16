import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { RolesPage } from './RolesPage';

function renderRolesPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <RolesPage />
    </QueryClientProvider>
  );
}

describe('RolesPage', () => {
  it('展示内置角色和权限分组', async () => {
    renderRolesPage();
    expect(await screen.findByText('角色权限')).toBeInTheDocument();
    expect((await screen.findAllByText('平台管理员')).length).toBeGreaterThan(0);
    expect(screen.getByLabelText('*:*')).toBeDisabled();

    await userEvent.click(screen.getByText('开发者'));
    expect(await screen.findByText('构建')).toBeInTheDocument();
    expect(screen.getByLabelText('build:create')).toBeChecked();
    await userEvent.click(screen.getByLabelText('build:create'));
    expect(screen.getByRole('button', { name: '保存权限' })).toBeEnabled();
  });
});
