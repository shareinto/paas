import { render, screen } from '@testing-library/react';
import { Button } from 'antd';
import { PageHeader } from './PageHeader';

test('页面头支持说明文案和稳定操作区样式', () => {
  render(<PageHeader title="应用" subtitle="统一管理应用交付单元" extra={<Button>创建应用</Button>} />);

  expect(screen.getByRole('heading', { name: '应用' })).toBeInTheDocument();
  expect(screen.getByText('统一管理应用交付单元')).toHaveClass('page-subtitle');
  expect(screen.getByText('创建应用').closest('.page-actions')).toBeInTheDocument();
});
