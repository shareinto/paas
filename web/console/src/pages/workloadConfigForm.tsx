import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { Button, Checkbox, Form, Input, InputNumber, Select, Space, Switch, Typography } from 'antd';
import { useEffect } from 'react';
import type { WorkloadStageConfig } from '../api';

const DEFAULT_NGINX_IMAGE = 'nginx:1.25-alpine';
const DEFAULT_NGINX_PORT = 80;

export function requiredLabel(label: string, required = false) {
  return required ? <span>{label} <span className="form-required-mark">*</span></span> : label;
}

export function WorkloadRuntimeFields() {
  return (
    <>
      <WorkloadRunConfigFields />
      <WorkloadAccessConfigFields />
      <WorkloadAdvancedFields />
    </>
  );
}

export function WorkloadRunConfigFields() {
  return (
    <>
      <Form.Item label="副本数" name="replicas"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item>
      <Form.Item label="容器端口" name="containerPort"><InputNumber min={1} max={65535} precision={0} style={{ width: '100%' }} /></Form.Item>
      <Form.Item label="服务端口" name="servicePort"><InputNumber min={1} max={65535} precision={0} style={{ width: '100%' }} /></Form.Item>
      <Form.Item label="CPU 请求" name="requestCpu"><Input placeholder="250m" /></Form.Item>
      <Form.Item label="内存请求" name="requestMemory"><Input placeholder="256Mi" /></Form.Item>
      <Form.Item label="CPU 限制" name="limitCpu"><Input placeholder="2" /></Form.Item>
      <Form.Item label="内存限制" name="limitMemory"><Input placeholder="2Gi" /></Form.Item>
      <Form.Item label="探针路径" name="probePath"><Input placeholder="/actuator/health" /></Form.Item>
      <Form.Item label="探针端口" name="probePort"><InputNumber min={1} max={65535} precision={0} style={{ width: '100%' }} /></Form.Item>
      <Form.Item label="存活初始等待" name="livenessInitialDelaySeconds"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item>
      <Form.Item label="就绪初始等待" name="readinessInitialDelaySeconds"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item>
      <Form.Item label="探针周期" name="probePeriodSeconds"><InputNumber min={1} precision={0} style={{ width: '100%' }} /></Form.Item>
      <Form.Item label="探针超时" name="probeTimeoutSeconds"><InputNumber min={1} precision={0} style={{ width: '100%' }} /></Form.Item>
    </>
  );
}

export function WorkloadAccessConfigFields() {
  return (
    <>
      <Form.Item label="服务类型" name="serviceType"><Select options={[{ value: 'ClusterIP', label: '集群内访问' }, { value: 'NodePort', label: '节点端口' }, { value: 'LoadBalancer', label: '负载均衡' }]} /></Form.Item>
      <Form.Item label="访问域名" name="domain"><Input placeholder="dev-order.example.com" /></Form.Item>
      <Form.Item label="Ingress Class" name="ingressClassName"><Input placeholder="higress" /></Form.Item>
      <Form.Item label="Ingress 路径" name="ingressPath"><Input placeholder="/" /></Form.Item>
      <Form.Item label="启用 TLS" name="ingressTls" valuePropName="checked"><Checkbox>为该域名启用 TLS</Checkbox></Form.Item>
      <Form.Item label="TLS Secret" name="ingressSecretName"><Input placeholder="order-dev-tls" /></Form.Item>
      <Form.Item label="证书签发器" name="ingressClusterIssuer"><Input placeholder="letsencrypt-prod" /></Form.Item>
    </>
  );
}

export function WorkloadApplicationConfigFields() {
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
          <Typography.Title level={5}>敏感配置引用</Typography.Title>
        </div>
        <Form.List name="secretRefs">
          {(fields, { add, remove }) => (
            <Space direction="vertical" size={10} className="full-width">
              {fields.map((field, index) => (
                <div className="workload-kv-row" key={field.key}>
                  <Form.Item label={`变量名 ${index + 1}`} name={[field.name, 'name']} rules={[{ required: true, message: '请输入变量名' }]}>
                    <Input aria-label={`敏感变量名 ${index + 1}`} placeholder="DB_PASSWORD" />
                  </Form.Item>
                  <Form.Item label={`Secret 引用 ${index + 1}`} name={[field.name, 'secretRef']} rules={[{ required: true, message: '请输入 Secret 引用' }]}>
                    <Input aria-label={`Secret 引用 ${index + 1}`} placeholder="secret/data/app/db" />
                  </Form.Item>
                  <Button aria-label={`删除敏感配置 ${index + 1}`} icon={<DeleteOutlined />} onClick={() => remove(field.name)} />
                </div>
              ))}
              <Button aria-label="添加敏感配置" icon={<PlusOutlined />} onClick={() => add({ name: '', secretRef: '' })}>添加敏感配置</Button>
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
                      <Checkbox>Base64 编码</Checkbox>
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

export function WorkloadNginxSidecarFields() {
  const form = Form.useFormInstance();
  const enabled = Form.useWatch('nginxSidecarEnabled', form);
  const containerPort = Form.useWatch('containerPort', form) || 8080;

  useEffect(() => {
    if (!enabled) return;
    const currentImage = form.getFieldValue('nginxSidecarImage');
    const currentPort = form.getFieldValue('nginxSidecarPort');
    const currentNginxConf = form.getFieldValue('nginxConf');
    const currentConfD = form.getFieldValue('nginxConfD');
    form.setFieldsValue({
      nginxSidecarImage: currentImage || DEFAULT_NGINX_IMAGE,
      nginxSidecarPort: currentPort || DEFAULT_NGINX_PORT,
      nginxConf: currentNginxConf || defaultNginxConf(),
      nginxConfD: Array.isArray(currentConfD) && currentConfD.length > 0 ? currentConfD : [{ fileName: 'default.conf', content: defaultNginxServerConf(Number(containerPort || 8080)) }]
    });
  }, [containerPort, enabled, form]);

  return (
    <section className="workload-config-section">
      <div className="workload-config-section-head">
        <Typography.Title level={5}>Nginx Sidecar</Typography.Title>
      </div>
      <Form.Item label="启用 Nginx Sidecar" name="nginxSidecarEnabled" valuePropName="checked">
        <Switch aria-label="启用 Nginx Sidecar" />
      </Form.Item>
      {enabled && (
        <>
          <Form.Item label={requiredLabel('Nginx 镜像', true)} name="nginxSidecarImage" rules={[{ required: true, message: '请输入 Nginx 镜像' }]}>
            <Input aria-label="Nginx 镜像" placeholder={DEFAULT_NGINX_IMAGE} />
          </Form.Item>
          <Form.Item label={requiredLabel('监听端口', true)} name="nginxSidecarPort" rules={[{ required: true, message: '请输入监听端口' }]}>
            <InputNumber aria-label="监听端口" min={1} max={65535} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label={requiredLabel('nginx.conf', true)} name="nginxConf" rules={[{ required: true, message: '请输入 nginx.conf 内容' }]}>
            <Input.TextArea aria-label="nginx.conf" rows={8} />
          </Form.Item>
          <div className="workload-config-section-head">
            <Typography.Title level={5}>conf.d 子配置</Typography.Title>
          </div>
          <Form.List name="nginxConfD">
            {(fields, { add, remove }) => (
              <Space direction="vertical" size={12} className="full-width">
                {fields.map((field, index) => (
                  <div className="workload-nginx-conf-row" key={field.key}>
                    <Form.Item
                      label={requiredLabel(`文件名 ${index + 1}`, true)}
                      name={[field.name, 'fileName']}
                      rules={[
                        { required: true, message: '请输入 conf.d 文件名' },
                        { pattern: /^[A-Za-z0-9._-]+\.conf$/, message: '文件名需以 .conf 结尾' }
                      ]}
                    >
                      <Input aria-label={`conf.d 文件名 ${index + 1}`} placeholder="default.conf" />
                    </Form.Item>
                    <Form.Item label={requiredLabel(`配置内容 ${index + 1}`, true)} name={[field.name, 'content']} rules={[{ required: true, message: '请输入 conf.d 配置内容' }]}>
                      <Input.TextArea aria-label={`conf.d 配置内容 ${index + 1}`} rows={7} />
                    </Form.Item>
                    <div className="workload-row-actions">
                      <Button aria-label={`删除 conf.d 配置 ${index + 1}`} icon={<DeleteOutlined />} onClick={() => remove(field.name)} />
                    </div>
                  </div>
                ))}
                <Button aria-label="添加 conf.d 配置" icon={<PlusOutlined />} onClick={() => add({ fileName: 'server.conf', content: defaultNginxServerConf(Number(containerPort || 8080)) })}>添加 conf.d 配置</Button>
              </Space>
            )}
          </Form.List>
        </>
      )}
    </section>
  );
}

export function ConfigValueLists() {
  return (
    <>
      <WorkloadApplicationConfigFields />
      <WorkloadAdvancedFields />
    </>
  );
}

export function WorkloadAdvancedFields() {
  return (
    <section className="workload-config-section">
      <div className="workload-config-section-head">
        <Typography.Title level={5}>高级配置</Typography.Title>
      </div>
      <Form.Item
        label="Values 覆盖 JSON"
        name="valuesOverrideText"
        rules={[{ validator: validateValuesOverride }]}
      >
        <Input.TextArea aria-label="Values 覆盖 JSON" rows={6} placeholder={'{"javaOpts":"-Xmx1024m","profile":"ltt"}'} />
      </Form.Item>
      <Form.Item label="终止等待秒数" name="terminationGracePeriodSeconds"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item>
    </section>
  );
}

export function workloadConfigPayload(values: any): Partial<WorkloadStageConfig> {
  const containerPort = Number(values.containerPort || 0);
  const servicePort = Number(values.servicePort || containerPort || 0);
  const probePort = Number(values.probePort || containerPort || 0);
  const probes = values.probePath && probePort ? [
    {
      name: 'liveness',
      type: 'http',
      path: values.probePath,
      port: probePort,
      initialDelaySeconds: Number(values.livenessInitialDelaySeconds ?? 20),
      periodSeconds: Number(values.probePeriodSeconds ?? 10),
      timeoutSeconds: Number(values.probeTimeoutSeconds ?? 1),
      failureThreshold: 3,
      successThreshold: 1
    },
    {
      name: 'readiness',
      type: 'http',
      path: values.probePath,
      port: probePort,
      initialDelaySeconds: Number(values.readinessInitialDelaySeconds ?? 10),
      periodSeconds: Number(values.probePeriodSeconds ?? 10),
      timeoutSeconds: Number(values.probeTimeoutSeconds ?? 1),
      failureThreshold: 5,
      successThreshold: 1
    }
  ] : [];
  const valuesOverride = parseValuesOverride(values.valuesOverrideText);
  if (values.serviceType && values.serviceType !== 'ClusterIP') valuesOverride.serviceType = values.serviceType;
  if (values.terminationGracePeriodSeconds !== undefined && values.terminationGracePeriodSeconds !== null) valuesOverride.terminationGracePeriodSeconds = Number(values.terminationGracePeriodSeconds);
  if (values.nginxSidecarEnabled) {
    valuesOverride.nginxSidecar = {
      enabled: true,
      image: values.nginxSidecarImage || DEFAULT_NGINX_IMAGE,
      port: Number(values.nginxSidecarPort || DEFAULT_NGINX_PORT),
      nginxConf: values.nginxConf || defaultNginxConf(),
      confD: cleanNginxConfD(values.nginxConfD),
      routeServiceToSidecar: true
    };
  } else {
    delete valuesOverride.nginxSidecar;
  }
  return {
    replicas: Number(values.replicas ?? 1),
    servicePorts: containerPort ? [{ name: 'http', port: servicePort || containerPort, targetPort: containerPort, protocol: 'TCP' }] : [],
    resourceRequests: { cpu: values.requestCpu || '', memory: values.requestMemory || '' },
    resourceLimits: { cpu: values.limitCpu || '', memory: values.limitMemory || '' },
    probes,
    ingressHosts: values.domain ? [{
      host: values.domain,
      path: values.ingressPath || '/',
      servicePort: 'http',
      tls: !!values.ingressTls,
      className: values.ingressClassName || '',
      pathType: 'Prefix',
      secretName: values.ingressSecretName || '',
      clusterIssuer: values.ingressClusterIssuer || ''
    }] : [],
    envVars: cleanEnvVars(values.envVars),
    secretRefs: cleanSecretRefs(values.secretRefs),
    configFiles: cleanConfigFiles(values.configFiles),
    writableDirs: cleanWritableDirs(values.writableDirs),
    valuesOverride
  };
}

export function workloadConfigFormValues(config?: WorkloadStageConfig) {
  const liveness = config?.probes?.find((probe) => probe.name === 'liveness');
  const readiness = config?.probes?.find((probe) => probe.name === 'readiness');
  const ingress = config?.ingressHosts?.[0];
  const valuesOverride = config?.valuesOverride || {};
  const nginxSidecar = normalizeNginxSidecar(valuesOverride.nginxSidecar);
  const containerPort = config?.servicePorts?.[0]?.targetPort || 8080;
  return {
    replicas: config?.replicas ?? 1,
    containerPort,
    servicePort: config?.servicePorts?.[0]?.port || 80,
    serviceType: String(valuesOverride.serviceType || 'ClusterIP'),
    domain: ingress?.host || '',
    ingressClassName: ingress?.className || '',
    ingressPath: ingress?.path || '/',
    ingressTls: !!ingress?.tls,
    ingressSecretName: ingress?.secretName || '',
    ingressClusterIssuer: ingress?.clusterIssuer || '',
    requestCpu: config?.resourceRequests?.cpu || '',
    requestMemory: config?.resourceRequests?.memory || '',
    limitCpu: config?.resourceLimits?.cpu || '',
    limitMemory: config?.resourceLimits?.memory || '',
    probePath: liveness?.path || readiness?.path || '',
    probePort: liveness?.port || readiness?.port || config?.servicePorts?.[0]?.targetPort || 8080,
    livenessInitialDelaySeconds: liveness?.initialDelaySeconds ?? 20,
    readinessInitialDelaySeconds: readiness?.initialDelaySeconds ?? 10,
    probePeriodSeconds: liveness?.periodSeconds || readiness?.periodSeconds || 10,
    probeTimeoutSeconds: liveness?.timeoutSeconds || readiness?.timeoutSeconds || 1,
    terminationGracePeriodSeconds: valuesOverride.terminationGracePeriodSeconds ?? 60,
    envVars: config?.envVars || [],
    secretRefs: config?.secretRefs || [],
    configFiles: config?.configFiles || [],
    writableDirs: config?.writableDirs || [],
    nginxSidecarEnabled: !!nginxSidecar.enabled,
    nginxSidecarImage: nginxSidecar.image || DEFAULT_NGINX_IMAGE,
    nginxSidecarPort: nginxSidecar.port || DEFAULT_NGINX_PORT,
    nginxConf: nginxSidecar.nginxConf || defaultNginxConf(),
    nginxConfD: nginxSidecar.confD?.length ? nginxSidecar.confD : [{ fileName: 'default.conf', content: defaultNginxServerConf(Number(containerPort || 8080)) }],
    valuesOverrideText: formatValuesOverride(valuesOverride)
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

function cleanSecretRefs(items?: Array<{ name?: string; secretRef?: string }>) {
  return (items || []).map((item) => ({ name: item.name?.trim() || '', secretRef: item.secretRef?.trim() || '' })).filter((item) => item.name && item.secretRef);
}

function cleanNginxConfD(items?: Array<{ fileName?: string; content?: string }>) {
  return (items || [])
    .map((item) => ({ fileName: item.fileName?.trim() || '', content: item.content || '' }))
    .filter((item) => item.fileName && item.content);
}

function normalizeNginxSidecar(raw: unknown) {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
    return {} as { enabled?: boolean; image?: string; port?: number; nginxConf?: string; confD?: Array<{ fileName: string; content: string }> };
  }
  const sidecar = raw as Record<string, any>;
  return {
    enabled: !!sidecar.enabled,
    image: typeof sidecar.image === 'string' ? sidecar.image : '',
    port: Number(sidecar.port || 0),
    nginxConf: typeof sidecar.nginxConf === 'string' ? sidecar.nginxConf : '',
    confD: Array.isArray(sidecar.confD) ? cleanNginxConfD(sidecar.confD) : []
  };
}

function defaultNginxConf() {
  return `worker_processes auto;

events {
  worker_connections 1024;
}

http {
  include /etc/nginx/mime.types;
  default_type application/octet-stream;
  sendfile on;
  keepalive_timeout 65;
  include /etc/nginx/conf.d/*.conf;
}`;
}

function defaultNginxServerConf(containerPort: number) {
  return `server {
  listen ${DEFAULT_NGINX_PORT};

  location / {
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_pass http://127.0.0.1:${containerPort || 8080};
  }
}`;
}

function parseValuesOverride(raw?: string) {
  const text = String(raw || '').trim();
  if (!text) return {} as Record<string, unknown>;
  try {
    const parsed = JSON.parse(text);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed as Record<string, unknown> : {};
  } catch {
    return {};
  }
}

function formatValuesOverride(values: Record<string, unknown>) {
  const copy = { ...values };
  delete copy.serviceType;
  delete copy.terminationGracePeriodSeconds;
  delete copy.nginxSidecar;
  return Object.keys(copy).length ? JSON.stringify(copy, null, 2) : '';
}

async function validateValuesOverride(_: unknown, raw?: string) {
  const text = String(raw || '').trim();
  if (!text) return;
  try {
    const parsed = JSON.parse(text);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error('invalid');
    }
  } catch {
    throw new Error('请输入合法的 JSON 对象');
  }
}
