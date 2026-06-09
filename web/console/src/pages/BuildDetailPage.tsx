import { PauseCircleOutlined, SearchOutlined } from '@ant-design/icons';
import { Button, Card, Descriptions, Input, Space, Switch } from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useParams } from 'react-router-dom';
import { streamBuildLog } from '../api';
import { PageHeader } from '../components/PageHeader';

const DEFAULT_VISIBLE_LOG_LINES = 500;

export function BuildDetailPage() {
  const { id = 'build_128' } = useParams();
  const [paused, setPaused] = useState(false);
  const [logText, setLogText] = useState('');
  const [showFullLog, setShowFullLog] = useState(false);
  const [streamStatus, setStreamStatus] = useState('connecting');
  const logRef = useRef<HTMLPreElement | null>(null);
  const statusText: Record<string, string> = {
    connecting: '连接中',
    streaming: '监听中',
    reconnecting: '重连中',
    error: '日志读取失败',
    paused: '已暂停',
    pending: '等待中',
    running: '构建中',
    succeeded: '构建成功',
    failed: '构建失败',
    aborted: '已取消',
    unstable: '构建不稳定'
  };
  const emptyLogText = streamStatus === 'error' ? '日志读取失败，请稍后重试' : '等待构建日志';
  const logView = useMemo(() => {
    if (!logText) return { displayText: '', hiddenLineCount: 0 };
    const lines = logText.split(/\r?\n/);
    const hiddenLineCount = Math.max(0, lines.length - DEFAULT_VISIBLE_LOG_LINES);
    return {
      displayText: showFullLog || hiddenLineCount === 0 ? logText : lines.slice(-DEFAULT_VISIBLE_LOG_LINES).join('\n'),
      hiddenLineCount
    };
  }, [logText, showFullLog]);
  useEffect(() => {
    setLogText('');
    setShowFullLog(false);
    setStreamStatus(paused ? 'paused' : 'connecting');
    if (paused || !id) return;
    return streamBuildLog(id, (chunk) => {
      setLogText((current) => current ? `${current}\n${chunk}` : chunk);
      setStreamStatus('streaming');
    }, setStreamStatus);
  }, [id, paused]);
  useEffect(() => {
    if (paused) return;
    const el = logRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [logText, paused]);
  const logActions = (
    <Space>
      <Input prefix={<SearchOutlined />} placeholder="搜索日志" />
      <Switch checkedChildren="只看错误" unCheckedChildren="全部" />
      {logView.hiddenLineCount > 0 && (
        <Button onClick={() => setShowFullLog((value) => !value)}>
          {showFullLog ? '收起日志' : '完整日志'}
        </Button>
      )}
      <Button icon={<PauseCircleOutlined />} onClick={() => setPaused((value) => !value)}>{paused ? '继续滚动' : '暂停滚动'}</Button>
    </Space>
  );
  return (
    <>
      <PageHeader title="构建详情" />
      <Card className="summary-card build-detail-summary">
        <Descriptions size="small" column={4} items={[
          { key: 'id', label: '构建编号', children: id },
          { key: 'status', label: '状态', children: statusText[streamStatus] || streamStatus },
          { key: 'ref', label: '构建引用', children: 'main' },
          { key: 'commit', label: '提交', children: '8c1a09f' }
        ]} />
      </Card>
      <Card className="build-log-card" title="实时日志" extra={logActions}>
        <div className="build-log-body">
          {logView.hiddenLineCount > 0 && !showFullLog && (
            <div className="build-log-truncated-tip">
              当前仅显示最近 {DEFAULT_VISIBLE_LOG_LINES} 行，已隐藏 {logView.hiddenLineCount} 行。
            </div>
          )}
          <pre ref={logRef} className="terminal-log">{logView.displayText || emptyLogText}</pre>
        </div>
      </Card>
    </>
  );
}
