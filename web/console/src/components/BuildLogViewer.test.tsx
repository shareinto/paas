import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterEach, expect, test, vi } from 'vitest';
import { BuildLogViewer } from './BuildLogViewer';
import { streamBuildLog } from '../api';

vi.mock('../api', () => ({
  streamBuildLog: vi.fn()
}));

const streamBuildLogMock = vi.mocked(streamBuildLog);

afterEach(() => {
  vi.clearAllMocks();
});

test('短构建日志直接显示完整内容', async () => {
  streamBuildLogMock.mockImplementation((_buildRunId, onLog) => {
    onLog('short line 1\nshort line 2');
    return () => undefined;
  });

  const { container } = render(<BuildLogViewer buildRunId="build_128" />);

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

  const { container } = render(<BuildLogViewer buildRunId="build_128" />);

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

test('ANSI 颜色和关键字兜底染色为日志片段添加 class', async () => {
  streamBuildLogMock.mockImplementation((_buildRunId, onLog) => {
    onLog('\u001b[31mERROR failed\u001b[0m\n[WARN] disk high\n[INFO] done\nSUCCESS pushed');
    return () => undefined;
  });

  const { container } = render(<BuildLogViewer buildRunId="build_128" />);

  expect(await screen.findByText('ERROR failed')).toHaveClass('log-color-red');
  expect(screen.getByText('[WARN] disk high')).toHaveClass('log-color-yellow');
  expect(screen.getByText('[INFO] done')).toHaveClass('log-color-blue');
  expect(screen.getByText('SUCCESS pushed')).toHaveClass('log-color-green');
  expect(container.querySelector('.terminal-log')).toHaveTextContent('ERROR failed');
});

test('日志流 queued 状态按构建中展示', async () => {
  streamBuildLogMock.mockImplementation((_buildRunId, _onLog, onStatus) => {
    onStatus?.('queued');
    return () => undefined;
  });

  render(<BuildLogViewer buildRunId="build_queued" />);

  expect(await screen.findByText('构建中')).toBeInTheDocument();
  expect(screen.queryByText('排队中')).not.toBeInTheDocument();
});

test('构建历史弹窗把滚动限制在日志显示框内部', () => {
  const css = readFileSync(resolve(process.cwd(), 'src/styles.css'), 'utf8');

  expect(css).toMatch(/\.build-history-layout\s*\{[^}]*height:\s*min\(560px,\s*calc\(100vh - 260px\)\)/);
  expect(css).toMatch(/\.build-history-layout\s*\{[^}]*overflow:\s*hidden/);
  expect(css).toMatch(/\.build-history-log-panel\s*\{[^}]*overflow:\s*hidden/);
  expect(css).toMatch(/\.build-log-viewer\s*\{[^}]*height:\s*100%/);
  expect(css).toMatch(/\.terminal-log\s*\{[^}]*overflow:\s*auto/);
});
