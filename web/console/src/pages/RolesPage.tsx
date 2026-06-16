import { SafetyCertificateOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Card, Checkbox, Empty, List, Space, Tag, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { listPermissions, listRoles, updateRolePermissions } from '../api';
import { PageHeader } from '../components/PageHeader';

const permissionGroups: Record<string, string> = {
  '*': '全部权限',
  tenant: '租户',
  project: '项目',
  cluster: '集群',
  application: '应用',
  stage: '阶段',
  build: '构建',
  freight: '版本货物',
  deployment: '发布',
  runtime: '运行时',
  audit: '审计',
  secret: '密钥'
};

const scopeNames: Record<string, string> = {
  platform: '平台',
  tenant: '租户',
  project: '项目',
  application: '应用',
  stage: '阶段'
};

export function RolesPage() {
  const queryClient = useQueryClient();
  const { data: roles = [], isLoading } = useQuery({ queryKey: ['roles'], queryFn: listRoles });
  const { data: permissions = [] } = useQuery({ queryKey: ['permissions'], queryFn: listPermissions });
  const [selectedRoleId, setSelectedRoleId] = useState<string>();
  const selectedRole = roles.find((role) => role.id === (selectedRoleId || roles[0]?.id));
  const [selectedPermissions, setSelectedPermissions] = useState<string[]>([]);
  const groupedPermissions = useMemo(() => groupPermissions(permissions), [permissions]);
  const savedPermissions = useMemo(() => [...(selectedRole?.permissions || [])].sort(), [selectedRole]);
  const dirty = selectedPermissions.join('\n') !== savedPermissions.join('\n');
  const saveMutation = useMutation({
    mutationFn: () => updateRolePermissions(selectedRole?.id || '', selectedPermissions),
    onSuccess: async (role) => {
      message.success('角色权限已保存');
      setSelectedPermissions([...role.permissions].sort());
      await queryClient.invalidateQueries({ queryKey: ['roles'] });
      await queryClient.invalidateQueries({ queryKey: ['permissions'] });
    }
  });

  useEffect(() => {
    setSelectedPermissions(savedPermissions);
  }, [savedPermissions]);

  return (
    <>
      <PageHeader title="角色权限" subtitle="查看并编辑平台内置角色的权限点，成员授权会按保存后的角色权限实时判定。" />
      <div className="roles-layout">
        <Card className="roles-list-card" loading={isLoading} title="内置角色">
          <List
            dataSource={roles}
            renderItem={(role) => (
              <List.Item className={role.id === selectedRole?.id ? 'role-list-item active' : 'role-list-item'} onClick={() => setSelectedRoleId(role.id)}>
                <Space direction="vertical" size={2}>
                  <Typography.Text strong>{role.name}</Typography.Text>
                  <Typography.Text className="muted">{role.id}</Typography.Text>
                </Space>
              </List.Item>
            )}
          />
        </Card>
        <Card className="roles-detail-card" loading={isLoading}>
          {selectedRole ? (
            <Space direction="vertical" size={16} className="roles-detail">
              <div className="roles-detail-head">
                <Space>
                  <SafetyCertificateOutlined />
                  <Typography.Title level={4}>{selectedRole.name}</Typography.Title>
                </Space>
                <Space wrap align="center">
                  {selectedRole.suggestedScopes.map((scope) => <Tag key={scope}>{scopeNames[scope] || scope}</Tag>)}
                  <Button onClick={() => setSelectedPermissions(savedPermissions)} disabled={!dirty || saveMutation.isPending}>重置</Button>
                  <Button type="primary" loading={saveMutation.isPending} disabled={!dirty || !selectedRole} onClick={() => saveMutation.mutate()}>保存权限</Button>
                </Space>
              </div>
              <Typography.Text className="muted">角色标识：{selectedRole.id}</Typography.Text>
              <div className="permission-groups">
                {groupedPermissions.map((group) => (
                  <section className="permission-group" key={group.key}>
                    <div className="permission-group-title">{group.title}</div>
                    <div className="permission-checkbox-grid">
                      {group.permissions.map((permission) => (
                        <Checkbox
                          key={permission}
                          checked={selectedPermissions.includes(permission)}
                          disabled={selectedRole.id === 'platform_admin' && permission === '*:*'}
                          onChange={(event) => togglePermission(permission, event.target.checked, setSelectedPermissions)}
                        >
                          {permission}
                        </Checkbox>
                      ))}
                    </div>
                  </section>
                ))}
              </div>
            </Space>
          ) : <Empty description="暂无角色" />}
        </Card>
      </div>
    </>
  );
}

function groupPermissions(permissions: string[]) {
  const groups = new Map<string, string[]>();
  permissions.forEach((permission) => {
    const [resource] = permission.split(':');
    const permissions = groups.get(resource) || [];
    permissions.push(permission);
    groups.set(resource, permissions);
  });
  return [...groups.entries()].map(([key, groupPermissions]) => ({ key, title: permissionGroups[key] || key, permissions: groupPermissions }));
}

function togglePermission(permission: string, checked: boolean, setSelectedPermissions: (updater: (current: string[]) => string[]) => void) {
  setSelectedPermissions((current) => {
    if (checked) return [...new Set([...current, permission])].sort();
    return current.filter((item) => item !== permission).sort();
  });
}
