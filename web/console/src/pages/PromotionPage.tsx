import { Button, Card, Descriptions, Steps, Table, Tag } from 'antd';
import { PageHeader } from '../components/PageHeader';

export function PromotionPage() {
  return (
    <>
      <PageHeader title="发布晋级" extra={<Button type="primary">创建晋级</Button>} />
      <Card>
        <Descriptions column={3} size="small" items={[
          { key: 'version', label: '版本', children: 'v1.8.2' },
          { key: 'digest', label: '镜像 digest', children: 'sha256:91ab' },
          { key: 'policy', label: '生产审批', children: '至少 1 人，禁止自审批' }
        ]} />
      </Card>
      <Card>
        <Steps current={2} items={[{ title: 'dev', status: 'finish' }, { title: 'test', status: 'finish' }, { title: 'staging', status: 'process' }, { title: 'prod', status: 'wait' }]} />
      </Card>
      <Table rowKey="id" pagination={false} dataSource={[{ id: 'promotion_18', env: 'prod', status: '待审批', approver: '生产审批人' }]} columns={[{ title: '发布', dataIndex: 'id' }, { title: '目标环境', dataIndex: 'env' }, { title: '状态', dataIndex: 'status', render: (v) => <Tag color="orange">{v}</Tag> }, { title: '审批人范围', dataIndex: 'approver' }, { title: '操作', render: () => <Button>回滚</Button> }]} />
    </>
  );
}
