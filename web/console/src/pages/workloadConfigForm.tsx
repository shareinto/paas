import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { Button, Checkbox, Form, Input, InputNumber, Space, Typography } from 'antd';
import type { WorkloadStageConfig } from '../api';

export function WorkloadRuntimeFields() {
  return (
    <>
      <Form.Item label="副本数" name="replicas"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item>
      <Form.Item label="容器端口" name="containerPort"><InputNumber min={1} max={65535} precision={0} style={{ width: '100%' }} /></Form.Item>
      <Form.Item label="Service 端口" name="servicePort"><InputNumber min={1} max={65535} precision={0} style={{ width: '100%' }} /></Form.Item>
      <Form.Item label="访问域名" name="domain"><Input placeholder="dev-order.example.com" /></Form.Item>
    </>
  );
}

export function ConfigValueLists() {
  return (
    <>
      <section className="workload-config-section">
        <div className="workload-config-section-head">
          <Typography.Title level={5}>环境变量</Typography.Title>
        </div>
        <Form.List name="envVars">
          {(fields, { add, remove }) => (
            <Space direction="vertical" size={10} className="full-width">
              {fields.map((field, index) => (
                <div className="workload-kv-row" key={field.key}>
                  <Form.Item label={`环境变量键 ${index + 1}`} name={[field.name, 'name']} rules={[{ required: true, message: '请输入环境变量键' }]}>
                    <Input aria-label={`环境变量键 ${index + 1}`} placeholder="SPRING_PROFILES_ACTIVE" />
                  </Form.Item>
                  <Form.Item label={`环境变量值 ${index + 1}`} name={[field.name, 'value']}>
                    <Input aria-label={`环境变量值 ${index + 1}`} placeholder="prod" />
                  </Form.Item>
                  <Button aria-label={`删除环境变量 ${index + 1}`} icon={<DeleteOutlined />} onClick={() => remove(field.name)} />
                </div>
              ))}
              <Button aria-label="添加环境变量" icon={<PlusOutlined />} onClick={() => add({ name: '', value: '' })}>添加环境变量</Button>
            </Space>
          )}
        </Form.List>
      </section>

      <section className="workload-config-section">
        <div className="workload-config-section-head">
          <Typography.Title level={5}>配置文件</Typography.Title>
        </div>
        <Form.List name="configFiles">
          {(fields, { add, remove }) => (
            <Space direction="vertical" size={12} className="full-width">
              {fields.map((field, index) => (
                <div className="workload-config-file-row" key={field.key}>
                  <Form.Item label={`配置文件路径 ${index + 1}`} name={[field.name, 'mountPath']} rules={[{ required: true, message: '请输入配置文件路径' }]}>
                    <Input aria-label={`配置文件路径 ${index + 1}`} placeholder="/etc/app/app.yaml" />
                  </Form.Item>
                  <Form.Item label={`配置文件内容 ${index + 1}`} name={[field.name, 'content']}>
                    <Input.TextArea aria-label={`配置文件内容 ${index + 1}`} rows={5} />
                  </Form.Item>
                  <div className="workload-row-actions">
                    <Form.Item name={[field.name, 'base64Encoded']} valuePropName="checked" noStyle>
                      <Checkbox>Base64</Checkbox>
                    </Form.Item>
                    <Button aria-label={`删除配置文件 ${index + 1}`} icon={<DeleteOutlined />} onClick={() => remove(field.name)} />
                  </div>
                </div>
              ))}
              <Button aria-label="添加配置文件" icon={<PlusOutlined />} onClick={() => add({ mountPath: '', content: '', base64Encoded: false })}>添加配置文件</Button>
            </Space>
          )}
        </Form.List>
      </section>

      <section className="workload-config-section">
        <div className="workload-config-section-head">
          <Typography.Title level={5}>可写目录</Typography.Title>
        </div>
        <Form.List name="writableDirs">
          {(fields, { add, remove }) => (
            <Space direction="vertical" size={10} className="full-width">
              {fields.map((field, index) => (
                <div className="workload-dir-row" key={field.key}>
                  <Form.Item label={`可写目录 ${index + 1}`} name={[field.name, 'mountPath']} rules={[{ required: true, message: '请输入可写目录' }]}>
                    <Input aria-label={`可写目录 ${index + 1}`} placeholder="/data" />
                  </Form.Item>
                  <Form.Item label={`目录属主 ${index + 1}`} name={[field.name, 'ownerGroup']}>
                    <Input aria-label={`目录属主 ${index + 1}`} placeholder="app:app" />
                  </Form.Item>
                  <Form.Item label={`目录权限 ${index + 1}`} name={[field.name, 'mode']}>
                    <Input aria-label={`目录权限 ${index + 1}`} placeholder="0775" />
                  </Form.Item>
                  <Button aria-label={`删除可写目录 ${index + 1}`} icon={<DeleteOutlined />} onClick={() => remove(field.name)} />
                </div>
              ))}
              <Button aria-label="添加可写目录" icon={<PlusOutlined />} onClick={() => add({ mountPath: '', ownerGroup: '', mode: '' })}>添加可写目录</Button>
            </Space>
          )}
        </Form.List>
      </section>
    </>
  );
}

export function workloadConfigPayload(values: any): Partial<WorkloadStageConfig> {
  const containerPort = Number(values.containerPort || 0);
  const servicePort = Number(values.servicePort || containerPort || 0);
  return {
    replicas: Number(values.replicas ?? 1),
    servicePorts: containerPort ? [{ name: 'http', port: servicePort || containerPort, targetPort: containerPort, protocol: 'TCP' }] : [],
    resourceRequests: {},
    resourceLimits: {},
    probes: [],
    ingressHosts: values.domain ? [{ host: values.domain, path: '/', servicePort: 'http', tls: false }] : [],
    envVars: cleanEnvVars(values.envVars),
    configFiles: cleanConfigFiles(values.configFiles),
    writableDirs: cleanWritableDirs(values.writableDirs)
  };
}

export function workloadConfigFormValues(config?: WorkloadStageConfig) {
  return {
    replicas: config?.replicas ?? 1,
    containerPort: config?.servicePorts?.[0]?.targetPort || 8080,
    servicePort: config?.servicePorts?.[0]?.port || 80,
    domain: config?.ingressHosts?.[0]?.host || '',
    envVars: config?.envVars || [],
    configFiles: config?.configFiles || [],
    writableDirs: config?.writableDirs || []
  };
}

function cleanEnvVars(items?: Array<{ name?: string; value?: string }>) {
  return (items || []).map((item) => ({ name: item.name?.trim() || '', value: item.value || '' })).filter((item) => item.name);
}

function cleanConfigFiles(items?: Array<{ mountPath?: string; content?: string; base64Encoded?: boolean }>) {
  return (items || []).map((item) => ({ mountPath: item.mountPath?.trim() || '', content: item.content || '', base64Encoded: !!item.base64Encoded })).filter((item) => item.mountPath);
}

function cleanWritableDirs(items?: Array<{ mountPath?: string; ownerGroup?: string; mode?: string; sizeLimit?: string }>) {
  return (items || []).map((item) => ({ mountPath: item.mountPath?.trim() || '', ownerGroup: item.ownerGroup?.trim() || '', mode: item.mode?.trim() || '', sizeLimit: item.sizeLimit || '' })).filter((item) => item.mountPath);
}
