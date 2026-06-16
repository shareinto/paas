import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { MemberManagement } from './MemberManagement';

function renderMemberManagement(scopeId = 'project_1') {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemberManagement scopeKind="project" scopeId={scopeId} title="项目成员" />
    </QueryClientProvider>
  );
}

async function visibleSelectOption(label: string) {
  await waitFor(() => {
    expect(
      Array.from(document.querySelectorAll('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option-content'))
        .some((item) => item.textContent === label)
    ).toBe(true);
  });
  return Array.from(document.querySelectorAll<HTMLElement>('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option-content'))
    .find((item) => item.textContent === label)!;
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

  it('添加项目成员时仅允许选择项目角色并禁用停用用户', async () => {
    renderMemberManagement();
    expect(await screen.findByText('李雷')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /添加成员/ }));
    await screen.findByLabelText('平台用户');

    await userEvent.click(screen.getByLabelText('成员角色'));
    expect(await visibleSelectOption('开发者')).toBeInTheDocument();
    expect(await visibleSelectOption('项目管理员')).toBeInTheDocument();
    expect(document.querySelector('.ant-select-dropdown:not(.ant-select-dropdown-hidden)')?.textContent).not.toContain('平台管理员');
    expect(document.querySelector('.ant-select-dropdown:not(.ant-select-dropdown-hidden)')?.textContent).not.toContain('租户管理员');
    await userEvent.click(await visibleSelectOption('开发者'));

    await userEvent.click(screen.getByLabelText('平台用户'));
    const disabledUser = await visibleSelectOption('赵六（zhaoliu）');
    expect(disabledUser.closest('.ant-select-item-option')).toHaveClass('ant-select-item-option-disabled');

    await userEvent.click(await visibleSelectOption('韩梅梅（hanmeimei）'));
    await userEvent.click(document.querySelector<HTMLButtonElement>('.ant-modal-footer .ant-btn-primary')!);

    expect(await screen.findByText('项目成员已保存；源码仓库权限需在源码仓库详情页手动同步')).toBeInTheDocument();
    expect(await screen.findByText('韩梅梅')).toBeInTheDocument();
  });

  it('移除项目成员后刷新列表并提示手动同步源码仓库权限', async () => {
    renderMemberManagement();
    const memberCell = await screen.findByText('王芳');
    const row = memberCell.closest('tr');
    expect(row).not.toBeNull();

    await userEvent.click(within(row as HTMLTableRowElement).getByText('移除'));
    await screen.findByText('确认移除该成员？移除后会影响其 PaaS 访问权限。');
    await userEvent.click(document.querySelector<HTMLButtonElement>('.ant-popconfirm-buttons .ant-btn-primary')!);

    expect(await screen.findByText('项目成员已移除；源码仓库权限需在源码仓库详情页手动同步')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText('王芳')).not.toBeInTheDocument());
  });
});
