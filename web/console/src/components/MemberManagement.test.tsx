import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { MemberManagement } from './MemberManagement';

function renderMemberManagement() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemberManagement scopeKind="project" scopeId="project_1" title="项目成员" />
    </QueryClientProvider>
  );
}

describe('MemberManagement', () => {
  it('展示项目成员并打开角色修改弹窗', async () => {
    renderMemberManagement();
    expect(await screen.findByText('项目成员')).toBeInTheDocument();
    expect(await screen.findByText('李雷')).toBeInTheDocument();
    expect(screen.getByText('项目管理员')).toBeInTheDocument();

    await userEvent.click(screen.getAllByText('修改角色')[0]);
    await waitFor(() => expect(screen.getByText('修改成员角色')).toBeInTheDocument());
    expect(screen.getByLabelText('平台用户')).toBeDisabled();
    expect(screen.getByLabelText('成员角色')).toBeInTheDocument();
  });
});
