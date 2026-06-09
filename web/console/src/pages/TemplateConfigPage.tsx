import Editor from '@monaco-editor/react';
import { CheckCircleFilled, ExclamationCircleFilled } from '@ant-design/icons';
import { Card, List, Space, Tag, Typography } from 'antd';
import { useState } from 'react';
import { PageHeader } from '../components/PageHeader';

const initial = `initContainers:
  - name: init-data
    command: ["sh", "-c", "mkdir -p /data && chmod 775 /data"]
volumeMounts:
  - name: data
    mountPath: /data
securityContext:
  runAsNonRoot: true`;

export function TemplateConfigPage() {
  const [content, setContent] = useState(initial);
  const errors = content.includes('privileged: true') ? ['不允许使用特权容器配置'] : [];
  return (
    <>
      <PageHeader title="部署模板配置" />
      <div className="template-grid">
        <Card className="summary-card" title="模板内容">
          <Editor height="560px" language="yaml" value={content} onChange={(value) => setContent(value || '')} options={{ minimap: { enabled: false }, fontSize: 13 }} />
        </Card>
        <Card className="summary-card" title="校验结果和版本记录">
          <Space direction="vertical" className="full-width">
            {errors.length === 0 ? <Tag icon={<CheckCircleFilled />} color="success">校验通过</Tag> : <Tag icon={<ExclamationCircleFilled />} color="error">校验失败</Tag>}
            <Typography.Text type="secondary">模板变更会生成版本并记录审计日志。</Typography.Text>
            <List size="small" dataSource={errors.length ? errors : ['initContainer 目录初始化配置有效', 'volumeMount 配置有效', 'securityContext 配置有效']} renderItem={(item) => <List.Item>{item}</List.Item>} />
            <List header="版本记录" size="small" dataSource={['v3 李雷 2026-05-30', 'v2 王芳 2026-05-29', 'v1 平台模板']} renderItem={(item) => <List.Item>{item}</List.Item>} />
          </Space>
        </Card>
      </div>
    </>
  );
}
