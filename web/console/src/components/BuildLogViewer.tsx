import { PauseCircleOutlined, SearchOutlined } from '@ant-design/icons';
import { Button, Input, Space, Switch, Typography } from 'antd';
import { ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { streamBuildLog } from '../api';

const DEFAULT_VISIBLE_LOG_LINES = 500;

type BuildLogViewerProps = {
  buildRunId?: string;
  className?: string;
  onStatusChange?: (status: string) => void;
};

type LogSegment = {
  text: string;
  className?: string;
};

const STATUS_TEXT: Record<string, string> = {
  connecting: '连接中',
  streaming: '监听中',
  reconnecting: '重连中',
  error: '日志读取失败',
  paused: '已暂停',
  pending: '等待中',
  queued: '构建中',
  running: '构建中',
  succeeded: '构建成功',
  failed: '构建失败',
  aborted: '已取消',
  unstable: '构建不稳定'
};

export function BuildLogViewer({ buildRunId, className = '', onStatusChange }: BuildLogViewerProps) {
  const [paused, setPaused] = useState(false);
  const [logText, setLogText] = useState('');
  const [showFullLog, setShowFullLog] = useState(false);
  const [streamStatus, setStreamStatus] = useState('pending');
  const [keyword, setKeyword] = useState('');
  const [errorsOnly, setErrorsOnly] = useState(false);
  const logRef = useRef<HTMLDivElement | null>(null);
  const statusChangeRef = useRef(onStatusChange);

  useEffect(() => {
    statusChangeRef.current = onStatusChange;
  }, [onStatusChange]);

  useEffect(() => {
    setLogText('');
    setShowFullLog(false);
    setStreamStatus(buildRunId ? (paused ? 'paused' : 'connecting') : 'pending');
    if (paused || !buildRunId) return;
    return streamBuildLog(buildRunId, (chunk) => {
      setLogText((current) => current ? `${current}\n${chunk}` : chunk);
      setStreamStatus('streaming');
    }, (status) => {
      setStreamStatus(status);
      statusChangeRef.current?.(status);
    });
  }, [buildRunId, paused]);

  const logView = useMemo(() => {
    const filteredText = filterLogText(logText, keyword, errorsOnly);
    if (!filteredText) return { displayText: '', hiddenLineCount: 0 };
    const lines = filteredText.split(/\r?\n/);
    const hiddenLineCount = Math.max(0, lines.length - DEFAULT_VISIBLE_LOG_LINES);
    return {
      displayText: showFullLog || hiddenLineCount === 0 ? filteredText : lines.slice(-DEFAULT_VISIBLE_LOG_LINES).join('\n'),
      hiddenLineCount
    };
  }, [errorsOnly, keyword, logText, showFullLog]);

  useEffect(() => {
    if (paused) return;
    const el = logRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [logView.displayText, paused]);

  const emptyLogText = streamStatus === 'error' ? '日志读取失败，请稍后重试' : buildRunId ? '等待构建日志' : '请选择一条构建记录查看日志。';

  return (
    <div className={`build-log-viewer ${className}`.trim()}>
      <div className="build-log-toolbar">
        <Space size={10} wrap>
          <Typography.Text strong>实时日志</Typography.Text>
          <Typography.Text type="secondary">{STATUS_TEXT[streamStatus] || streamStatus}</Typography.Text>
        </Space>
        <Space wrap>
          <Input prefix={<SearchOutlined />} placeholder="搜索日志" value={keyword} onChange={(event) => setKeyword(event.target.value)} allowClear />
          <Switch checked={errorsOnly} onChange={setErrorsOnly} checkedChildren="只看错误" unCheckedChildren="全部" />
          {logView.hiddenLineCount > 0 && (
            <Button onClick={() => setShowFullLog((value) => !value)}>
              {showFullLog ? '收起日志' : '完整日志'}
            </Button>
          )}
          <Button icon={<PauseCircleOutlined />} onClick={() => setPaused((value) => !value)}>
            {paused ? '继续滚动' : '暂停滚动'}
          </Button>
        </Space>
      </div>
      <div className="build-log-body">
        {logView.hiddenLineCount > 0 && !showFullLog && (
          <div className="build-log-truncated-tip">
            当前仅显示最近 {DEFAULT_VISIBLE_LOG_LINES} 行，已隐藏 {logView.hiddenLineCount} 行。
          </div>
        )}
        <div ref={logRef} className="terminal-log" role="log">
          {logView.displayText ? renderLogText(logView.displayText) : <span>{emptyLogText}</span>}
        </div>
      </div>
    </div>
  );
}

function filterLogText(logText: string, keyword: string, errorsOnly: boolean) {
  const normalizedKeyword = keyword.trim().toLowerCase();
  if (!normalizedKeyword && !errorsOnly) return logText;
  return logText.split(/\r?\n/).filter((line) => {
    const lower = stripAnsi(line).toLowerCase();
    if (normalizedKeyword && !lower.includes(normalizedKeyword)) return false;
    if (errorsOnly && !/(error|fail|failed|failure|失败|错误|异常)/i.test(lower)) return false;
    return true;
  }).join('\n');
}

function renderLogText(text: string) {
  const lines = text.split(/\r?\n/);
  return lines.map((line, lineIndex) => (
    <span key={lineIndex} className="terminal-log-line">
      {renderLogLine(line)}
      {lineIndex < lines.length - 1 && '\n'}
    </span>
  ));
}

function renderLogLine(line: string): ReactNode {
  const ansiSegments = parseAnsiSegments(line);
  if (ansiSegments.some((segment) => segment.className)) {
    return ansiSegments.map((segment, index) => (
      <span key={index} className={segment.className}>{segment.text}</span>
    ));
  }
  return <span className={fallbackLogClass(line)}>{line}</span>;
}

function parseAnsiSegments(line: string): LogSegment[] {
  const segments: LogSegment[] = [];
  const ansiPattern = /\x1b\[([0-9;]*)m/g;
  let currentClass = '';
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  while ((match = ansiPattern.exec(line)) !== null) {
    if (match.index > lastIndex) {
      segments.push({ text: line.slice(lastIndex, match.index), className: currentClass || undefined });
    }
    currentClass = ansiClassForCodes(match[1], currentClass);
    lastIndex = ansiPattern.lastIndex;
  }
  if (lastIndex < line.length) {
    segments.push({ text: line.slice(lastIndex), className: currentClass || undefined });
  }
  return segments.length ? segments : [{ text: line }];
}

function ansiClassForCodes(rawCodes: string, currentClass: string) {
  const codes = rawCodes.split(';').filter(Boolean).map((code) => Number(code));
  if (codes.length === 0 || codes.includes(0)) return '';
  if (codes.includes(31) || codes.includes(91)) return 'log-color-red';
  if (codes.includes(32) || codes.includes(92)) return 'log-color-green';
  if (codes.includes(33) || codes.includes(93)) return 'log-color-yellow';
  if (codes.includes(34) || codes.includes(94)) return 'log-color-blue';
  if (codes.includes(35) || codes.includes(95)) return 'log-color-magenta';
  if (codes.includes(36) || codes.includes(96)) return 'log-color-cyan';
  return currentClass;
}

function fallbackLogClass(line: string) {
  if (/(error|fail|failed|failure|失败|错误|异常)/i.test(line)) return 'log-color-red';
  if (/(warn|warning|警告)/i.test(line)) return 'log-color-yellow';
  if (/(success|succeeded|成功|完成)/i.test(line)) return 'log-color-green';
  if (/(info|\[INFO\])/i.test(line)) return 'log-color-blue';
  return undefined;
}

function stripAnsi(text: string) {
  return text.replace(/\x1b\[[0-9;]*m/g, '');
}
