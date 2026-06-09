import { ConfigProvider } from 'antd';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, expect, test, vi } from 'vitest';
import { BuildDetailPage } from './BuildDetailPage';
import { streamBuildLog } from '../api';

vi.mock('../api', () => ({
  streamBuildLog: vi.fn()
}));

const streamBuildLogMock = vi.mocked(streamBuildLog);

function renderBuildDetail() {
  return render(
    <ConfigProvider>
      <MemoryRouter initialEntries={['/builds/build_128']}>
        <Routes>
          <Route path="/builds/:id" element={<BuildDetailPage />} />
        </Routes>
      </MemoryRouter>
    </ConfigProvider>
  );
}

afterEach(() => {
  vi.clearAllMocks();
});

test('短构建日志直接显示完整内容', async () => {
  streamBuildLogMock.mockImplementation((_buildRunId, onLog) => {
    onLog('short line 1\nshort line 2');
    return () => undefined;
  });

  const { container } = renderBuildDetail();

  await waitFor(() => expect(container.querySelector('.terminal-log')).toHaveTextContent('short line 1'));
  expect(container.querySelector('.terminal-log')).toHaveTextContent('short line 2');
  expect(screen.queryByText(/当前仅显示最近 500 行/)).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '完整日志' })).not.toBeInTheDocument();
});

test('长构建日志默认只显示最近 500 行并可切换完整日志', async () => {
  const lines = Array.from({ length: 505 }, (_, index) => `line-${String(index + 1).padStart(4, '0')}`);
  streamBuildLogMock.mockImplementation((_buildRunId, onLog) => {
    onLog(lines.join('\n'));
    return () => undefined;
  });

  const { container } = renderBuildDetail();

  expect(await screen.findByText('当前仅显示最近 500 行，已隐藏 5 行。')).toBeInTheDocument();
  const log = container.querySelector('.terminal-log');
  expect(log).toHaveTextContent('line-0006');
  expect(log).toHaveTextContent('line-0505');
  expect(log).not.toHaveTextContent('line-0001');

  await userEvent.click(screen.getByRole('button', { name: '完整日志' }));

  await waitFor(() => expect(log).toHaveTextContent('line-0001'));
  expect(log).toHaveTextContent('line-0505');
  expect(screen.queryByText('当前仅显示最近 500 行，已隐藏 5 行。')).not.toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: '收起日志' }));

  expect(await screen.findByText('当前仅显示最近 500 行，已隐藏 5 行。')).toBeInTheDocument();
  expect(log).not.toHaveTextContent('line-0001');
});
