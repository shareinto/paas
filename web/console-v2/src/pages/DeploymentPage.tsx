import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type DragEvent, type ReactNode } from 'react';
import {
  Background,
  BaseEdge,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  getBezierPath,
  type Edge,
  type EdgeProps,
  type Node,
  type NodeProps
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  Archive,
  Box,
  CheckCircle2,
  ChevronRight,
  CircleDashed,
  Eye,
  EyeOff,
  FileText,
  FileArchive,
  GitBranch,
  GitPullRequestArrow,
  Globe2,
  HeartPulse,
  HelpCircle,
  Layers3,
  PanelRightOpen,
  PackageOpen,
  PauseCircle,
  Plus,
  RefreshCw,
  RotateCcw,
  Rocket,
  Server,
  Search,
  Settings2,
  ShieldCheck,
  Terminal,
  Trash2,
  Workflow,
  X
} from 'lucide-react';
import {
  deliveryTopology,
  freights as initialFreights,
  versionSourceConfig as initialVersionSourceConfig,
  versionSourcePipelines,
  type ContainerImageSource,
  type Status,
  type VersionSourcePipeline,
  type VersionSourceWorkloadConfig,
  type WorkloadProbeConfig
} from '../data/mock';
import {
  approveApprovalTask,
  checkRuntimePodLogs,
  checkRuntimePodTerminal,
  completeStageVerification,
  createFreightFromVersionSource,
  createStagePromotion,
  getDeploymentHistoryDetail,
  getApprovalTask,
  getConfigDiff,
  getPublishTask,
  loadDeploymentPageBundle,
  listDeploymentHistory,
  listApprovalTasks,
  listPublishTasks,
  publishTask,
  redeployWithConfig,
  rejectApprovalTask,
  rejectPublishTask,
  restartRuntimeResource,
  type ApprovalTaskDetail,
  type ApprovalTaskSummary,
  type DeploymentEdge,
  type DeploymentFreight,
  type DeploymentFreightContainer,
  type DeploymentHistoryDetail,
  type DeploymentHistoryItem,
  type DeploymentStage,
  type DeploymentTopology,
  type PublishGateSummary
} from '../api/deployments';
import {
  cancelVersionSourcePipelineBuild,
  createVersionSourcePipeline,
  deleteVersionSourcePipeline,
  streamBuildRunLog,
  triggerVersionSourcePipelineBuild,
  updateVersionSourcePipeline,
  type PipelineEnvironmentOption
} from '../api/buildPipelines';
import { saveVersionSourceWorkloads, loadWorkloadStageConfig, saveWorkloadStageConfig } from '../api/workloads';
import { paasWS, type WSMessage } from '../api/ws';
import { StatusBadge } from '../components/StatusBadge';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { usePlatformSelection } from '../contexts/PlatformSelectionContext';
import { cn } from '../lib/utils';

type Stage = DeploymentStage;
type Freight = DeploymentFreight;
type ArgoHealthStatus = 'Healthy' | 'Progressing' | 'Degraded' | 'Suspended' | 'Missing' | 'Unknown';
type ArgoSyncStatus = 'Synced' | 'OutOfSync' | 'Unknown';
type VersionSourceConfig = {
  updatedAt: string;
  freightSerial: number;
  workloads: VersionSourceWorkloadConfig[];
};

type StageNodeData = {
  stage: Stage;
  freight: Freight | null;
  freightDisplayName: string;
  dragging: boolean;
  canDrop: boolean;
  dragOver: boolean;
  verificationDone: boolean;
  onDragEnterStage: (stageId: string) => void;
  onDragLeaveStage: (stageId: string) => void;
  onDropFreight: (stage: Stage, freightId: string) => void;
  onOpenRuntime: (stage: Stage) => void;
  onOpenConfig: (stage: Stage) => void;
  onOpenHistory: (stage: Stage) => void;
  onOpenVerification: (stage: Stage) => void;
  canVerify: boolean;
  publishPendingCount: number;
  canPublishPending: boolean;
  onOpenPublish: (stageId: string) => void;
  onOpenConfigDiff: (stage: Stage) => void;
};
type WarehouseNodeData = {
  config: VersionSourceConfig;
  dirty: boolean;
  onRefresh: () => void;
  onOpenConfig: () => void;
};
type PipelineNodeData = {
  pipeline: VersionSourcePipeline;
  referenced: boolean;
  onOpenConfig: (pipeline: VersionSourcePipeline) => void;
  onOpenBuild: (pipeline: VersionSourcePipeline) => void;
  onDelete: (pipeline: VersionSourcePipeline) => void;
};
type StageFlowNode = Node<StageNodeData, 'stageNode'>;
type WarehouseFlowNode = Node<WarehouseNodeData, 'warehouseNode'>;
type PipelineFlowNode = Node<PipelineNodeData, 'pipelineNode'>;
type ApprovalEdge = Edge<Record<string, never>, 'approvalEdge'>;

const nodeTypes = { stageNode: StageFlowNodeCard, warehouseNode: WarehouseNodeCard, pipelineNode: PipelineNodeCard };
const edgeTypes = { approvalEdge: ApprovalGateEdge };
const FLOW_COLUMN_WIDTH = 390;
const FLOW_ROW_HEIGHT = 260;
const FLOW_CARD_WIDTH = 300;
const FLOW_CARD_ESTIMATED_HEIGHT = 300;
const PIPELINE_NODE_SPACING = 230;
const WAREHOUSE_NODE_ID = 'version-source';
const DEPLOYMENT_DIALOG_CLASS = 'flex h-[calc(100vh-32px)] min-h-[800px] w-[1200px] max-w-[calc(100vw-32px)] flex-col overflow-hidden rounded-lg border bg-card shadow-2xl';

const stagePalette = {
  'geek-blue': { label: '极客蓝', hex: '#0072B2', fg: '#073B5F' },
  'sky-blue': { label: '天青蓝', hex: '#56B4E9', fg: '#075985' },
  'mint-green': { label: '薄荷绿', hex: '#009E73', fg: '#064E3B' },
  'turquoise-green': { label: '松石绿', hex: '#44AA99', fg: '#134E4A' },
  'lemon-yellow': { label: '柠檬黄', hex: '#F0E442', fg: '#5F4B00' },
  'amber-orange': { label: '琥珀橙', hex: '#E69F00', fg: '#6B3F00' },
  'rust-orange': { label: '铁锈红', hex: '#D55E00', fg: '#7C2D12' },
  'wine-purple': { label: '酒红紫', hex: '#882255', fg: '#5B1238' },
  'lilac-purple': { label: '丁香紫', hex: '#CC79A7', fg: '#701A4B' },
  'smoky-blue': { label: '烟熏蓝', hex: '#77AADD', fg: '#1E3A5F' },
  unpublished: { label: '未发布', hex: '#94A3B8', fg: '#475569' }
} as const;

type StageColorToken = keyof typeof stagePalette;
type StageTone = (typeof stagePalette)[StageColorToken] & { token: StageColorToken };

function inferStageColorToken(stage?: Stage): StageColorToken {
  if (!stage) return 'unpublished';
  if ('colorToken' in stage && typeof stage.colorToken === 'string' && stage.colorToken in stagePalette) {
    return stage.colorToken as StageColorToken;
  }
  const signature = `${stage.key} ${stage.name}`.toLowerCase();
  if (signature.includes('feature') || signature.includes('特性')) return 'sky-blue';
  if (signature.includes('dev') || signature.includes('开发')) return 'geek-blue';
  if (signature.includes('qa') || signature.includes('质检')) return 'lilac-purple';
  if (signature.includes('integration') || signature.includes('集成')) return 'mint-green';
  if (signature.includes('test') || signature.includes('测试')) return 'mint-green';
  if (signature.includes('dr') || signature.includes('灾备') || signature.includes('旁路')) return 'turquoise-green';
  if (signature.includes('pre') || signature.includes('预发')) return 'lemon-yellow';
  if (signature.includes('canary') || signature.includes('灰度')) return 'amber-orange';
  if (signature.includes('prod') || signature.includes('生产')) return 'wine-purple';
  return 'smoky-blue';
}

function toneForStage(stage?: Stage) {
  const token = inferStageColorToken(stage);
  return { ...stagePalette[token], token };
}

function toneStyle(tone: StageTone) {
  return {
    '--stage-color': tone.hex,
    '--stage-fg': tone.fg,
    '--stage-card-bg': 'hsl(var(--card))',
    '--stage-card-border': 'hsl(var(--border))',
    '--stage-chip-bg': `color-mix(in srgb, ${tone.hex} 10%, white)`,
    '--stage-chip-border': `color-mix(in srgb, ${tone.hex} 36%, white)`
  } as CSSProperties & Record<string, string>;
}

function freightIdFromDrag(event: DragEvent<HTMLElement>) {
  return event.dataTransfer.getData('application/x-paas-freight-id') || event.dataTransfer.getData('text/plain');
}

function deepestStageForFreight(freight: Freight, stages = deliveryTopology.stages) {
  return freight.currentStages
    .map((stageId) => stages.find((stage) => stage.id === stageId || stage.key === stageId))
    .filter((stage): stage is Stage => Boolean(stage))
    .sort((a, b) => b.lane - a.lane)[0];
}

function stagesByIds(stageIds: string[], stages: Stage[]) {
  const unique = [...new Set(stageIds)];
  return unique
    .map((stageId) => stages.find((stage) => stage.id === stageId || stage.key === stageId))
    .filter((stage): stage is Stage => Boolean(stage))
    .sort((a, b) => (a.lane - b.lane) || (a.row - b.row) || a.name.localeCompare(b.name));
}

function versionFromPipeline(pipeline?: VersionSourcePipeline) {
  const success = latestSuccessfulBuild(pipeline);
  return success?.version || '暂无版本';
}

function imageForContainer(container: VersionSourceWorkloadConfig['containers'][number], pipelines: VersionSourcePipeline[]) {
  if (container.imageSource.mode === 'custom') return container.imageSource.customImage || '未配置镜像';
  const pipeline = pipelines.find((item) => item.id === container.imageSource.pipelineId);
  return `registry.local/${container.id.replace(/-(main|sidecar)$/, '')}:${versionFromPipeline(pipeline)}`;
}

function versionForContainer(container: VersionSourceWorkloadConfig['containers'][number], pipelines: VersionSourcePipeline[]) {
  if (container.imageSource.mode === 'pipeline') {
    return versionFromPipeline(pipelines.find((item) => item.id === container.imageSource.pipelineId));
  }
  return (container.imageSource.customImage || 'custom').split(':').slice(-1)[0] || 'custom';
}

function latestSuccessfulBuild(pipeline?: VersionSourcePipeline) {
  return pipeline?.buildHistory.find((run) => run.status === 'healthy');
}

function normalizeFreightContainerName(name: string) {
  return name.trim() || 'app';
}

function versionSourceFingerprint(config: VersionSourceConfig, pipelines: VersionSourcePipeline[]) {
  return config.workloads.map((workload) => {
    const containers = workload.containers.map((container) => {
      if (container.imageSource.mode === 'custom') {
        return `${workload.id}:${container.name}:custom:${container.imageSource.customImage || ''}`;
      }
      const pipeline = pipelines.find((item) => item.id === container.imageSource.pipelineId);
      const version = latestSuccessfulBuild(pipeline)?.version || 'NO_SUCCESS';
      return `${workload.id}:${container.name}:pipeline:${version}`;
    }).sort();
    return containers.join(',');
  }).sort().join('|');
}

function versionSourceConsumedByFreights(
  config: VersionSourceConfig,
  pipelines: VersionSourcePipeline[],
  freights: Freight[],
  sourceFingerprint: string
) {
  return freights.some((freight) => {
    if (sourceFingerprint && freight.sourceFingerprint === sourceFingerprint) return true;
    return config.workloads.every((workload) => {
      const freightWorkload = freight.workloads.find((item) => item.name === workload.id);
      if (!freightWorkload) return false;
      return workload.containers.every((container) => {
        const containerName = normalizeFreightContainerName(container.name);
        const freightContainer = containersForFreightWorkload(freightWorkload).find((item) => normalizeFreightContainerName(item.name) === containerName);
        if (!freightContainer) return false;
        if (container.imageSource.mode === 'custom') {
          const expectedImage = (container.imageSource.customImage || '').trim();
          return Boolean(expectedImage) && freightContainer.image === expectedImage;
        }
        const pipeline = pipelines.find((item) => item.id === container.imageSource.pipelineId);
        const latest = latestSuccessfulBuild(pipeline);
        if (!latest) return false;
        const expectedVersions = [latest.version, latest.id].filter(Boolean);
        return expectedVersions.some((version) => (
          freightContainer.version === version ||
          freightContainer.version.includes(version) ||
          freightContainer.image.includes(`:${version}`) ||
          freightContainer.image.includes(version) ||
          freight.sourceFingerprint?.includes(version)
        ));
      });
    });
  });
}

function versionSourceMissingSuccessfulPipelines(config: VersionSourceConfig, pipelines: VersionSourcePipeline[]) {
  const missing = new Set<string>();
  config.workloads.forEach((workload) => {
    workload.containers.forEach((container) => {
      if (container.imageSource.mode !== 'pipeline' || !container.imageSource.pipelineId) return;
      const pipeline = pipelines.find((item) => item.id === container.imageSource.pipelineId);
      if (!latestSuccessfulBuild(pipeline)) {
        missing.add(pipeline?.name || container.imageSource.pipelineId);
      }
    });
  });
  return Array.from(missing);
}

function versionSourceHasRequiredSuccessfulVersions(config: VersionSourceConfig, pipelines: VersionSourcePipeline[]) {
  return versionSourceMissingSuccessfulPipelines(config, pipelines).length === 0;
}

function pipelineForContainer(container: VersionSourceWorkloadConfig['containers'][number], pipelines: VersionSourcePipeline[]) {
  if (container.imageSource.mode === 'custom') return '自定义镜像';
  return pipelines.find((item) => item.id === container.imageSource.pipelineId)?.name || '关联流水线';
}

function compactFreightTimestamp(date = new Date()) {
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}-${pad(date.getHours())}${pad(date.getMinutes())}${pad(date.getSeconds())}`;
}

function freightDisplayTime(date = new Date()) {
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function freightSortTime(freight: Freight) {
  const fromCreatedAt = Date.parse((freight.createdAt || '').replace(' ', 'T'));
  if (!Number.isNaN(fromCreatedAt)) return fromCreatedAt;
  const fromId = Date.parse(freight.id.replace(/^freight-/, '').replace('.', 'T'));
  if (!Number.isNaN(fromId)) return fromId;
  return 0;
}

function sortFreightsNewestFirst(freights: Freight[]) {
  return freights.slice().sort((a, b) => {
    const byTime = freightSortTime(b) - freightSortTime(a);
    if (byTime !== 0) return byTime;
    return b.id.localeCompare(a.id);
  });
}

function nextFreightFromVersionSource(config: VersionSourceConfig, pipelines: VersionSourcePipeline[], serial: number, sourceFingerprint: string): Freight {
  const now = new Date();
  const version = compactFreightTimestamp(now);
  return {
    id: version,
    name: version,
    createdAt: freightDisplayTime(now),
    source: '版本源刷新',
    sourceFingerprint,
    commit: `vs-${serial}`,
    currentStages: [],
    eligibleStages: ['dev'],
    workloads: config.workloads.map((workload) => {
      const containers = workload.containers.map((container) => {
        const version = versionForContainer(container, pipelines);
        return {
          name: container.name,
          pipeline: pipelineForContainer(container, pipelines),
          version,
          image: imageForContainer(container, pipelines),
          digest: `sha256:vs${serial}${workload.id.slice(0, 3)}${container.name.slice(0, 3)}`,
          status: 'healthy' as const
        };
      });
      const mainContainer = containers[0];
      return {
        name: workload.id,
        displayName: workload.name,
        pipeline: containers.length > 1 ? `${containers.length} 个镜像` : mainContainer?.pipeline || '版本源',
        version: containers.length > 1 ? `${containers.length} 个版本` : mainContainer?.version || '-',
        image: containers.length > 1 ? `${containers.length} 个镜像` : mainContainer?.image || '-',
        digest: containers.length > 1 ? `${containers.length} 个 digest` : mainContainer?.digest || '-',
        status: 'healthy' as const,
        containers
      };
    })
  };
}

function freightDisplayName(freight: Pick<Freight, 'id' | 'name'>) {
  return freight.name || freight.id;
}

export function DeploymentPage() {
  const { currentTenant, currentProject, currentApplication } = usePlatformSelection();
  const [topology, setTopology] = useState<DeploymentTopology>(deliveryTopology);
  const [deploymentSource, setDeploymentSource] = useState<'api' | 'mock'>('mock');
  const [deploymentError, setDeploymentError] = useState('');
  const [deploymentLoading, setDeploymentLoading] = useState(false);
  const [query, setQuery] = useState('');
  const [freightItems, setFreightItems] = useState<Freight[]>(initialFreights);
  const [versionSource, setVersionSource] = useState<VersionSourceConfig>(initialVersionSourceConfig);
  const [sourcePipelines, setSourcePipelines] = useState<VersionSourcePipeline[]>(versionSourcePipelines);
  const [pipelineSource, setPipelineSource] = useState<'api' | 'mock'>('mock');
  const [pipelineError, setPipelineError] = useState('');
  const [pipelineLoading, setPipelineLoading] = useState(false);
  const [pipelineOptionSource, setPipelineOptionSource] = useState<'api' | 'mock'>('mock');
  const [runtimeEnvironmentOptions, setRuntimeEnvironmentOptions] = useState<PipelineEnvironmentOption[]>(fallbackRuntimeEnvironmentOptions);
  const [buildEnvironmentOptions, setBuildEnvironmentOptions] = useState<PipelineEnvironmentOption[]>(fallbackBuildEnvironmentOptions);
  const [workloadSource, setWorkloadSource] = useState<'api' | 'mock'>('mock');
  const [workloadError, setWorkloadError] = useState('');
  const [workloadLoading, setWorkloadLoading] = useState(false);
  const [versionSourceMessage, setVersionSourceMessage] = useState('');
  const [freightSerial, setFreightSerial] = useState(initialVersionSourceConfig.freightSerial);
  const [activeFreightId, setActiveFreightId] = useState(initialFreights[0].id);
  const [draggingFreightId, setDraggingFreightId] = useState<string | null>(null);
  const [dragOverStageId, setDragOverStageId] = useState<string | null>(null);
  const [confirmTargetId, setConfirmTargetId] = useState<string | null>(null);
  const [runtimeStageId, setRuntimeStageId] = useState<string | null>(null);
  const [configStageId, setConfigStageId] = useState<string | null>(null);
  const [versionSourceOpen, setVersionSourceOpen] = useState(false);
  const [pipelineDialogOpen, setPipelineDialogOpen] = useState(false);
  const [configDiffStage, setConfigDiffStage] = useState<Stage | null>(null);
  const [pipelineConfigId, setPipelineConfigId] = useState<string | null>(null);
  const [pipelineBuildId, setPipelineBuildId] = useState<string | null>(null);
  const [approvalStageId, setApprovalStageId] = useState<string | null>(null);
  const [publishStageId, setPublishStageId] = useState<string | null>(null);
  const [historyStageId, setHistoryStageId] = useState<string | null>(null);
  const [publishGates, setPublishGates] = useState<PublishGateSummary[]>([]);
  const [verificationStageId, setVerificationStageId] = useState<string | null>(null);
  const [showPipelines, setShowPipelines] = useState(true);
  const [verifiedStageKeys, setVerifiedStageKeys] = useState<Set<string>>(() => new Set());
  const [configWorkloadId, setConfigWorkloadId] = useState(deliveryTopology.stages[0].configurableWorkloadIds[0]);
  const [promotionAutoPublish, setPromotionAutoPublish] = useState(true);
  const buildRefreshRef = useRef(false);

  useEffect(() => {
    let cancelled = false;
    setDeploymentLoading(true);
    setPipelineLoading(true);
    setWorkloadLoading(true);
    loadDeploymentPageBundle(currentApplication.id)
      .then((bundle) => {
        if (cancelled) return;
        const { workspace, pipelines, pipelineOptions, workloads } = bundle;
        const sortedFreights = sortFreightsNewestFirst(workspace.freights);
        setTopology(workspace.topology);
        setFreightItems(sortedFreights);
        setPublishGates(workspace.publishGates || []);
        setDeploymentSource(workspace.source);
        setDeploymentError(workspace.error || '');
        setActiveFreightId(sortedFreights[0]?.id || '');
        setRuntimeStageId(null);
        setConfigWorkloadId(workspace.topology.stages[0]?.configurableWorkloadIds[0] || '');
        setVerifiedStageKeys(verifiedKeysFromStages(workspace.topology.stages));
        setSourcePipelines(pipelines.pipelines);
        setPipelineSource(pipelines.source);
        setPipelineOptionSource(pipelineOptions.source);
        setRuntimeEnvironmentOptions(pipelineOptions.runtimeOptions.length ? pipelineOptions.runtimeOptions : fallbackRuntimeEnvironmentOptions);
        setBuildEnvironmentOptions(pipelineOptions.buildEnvironmentOptions.length ? pipelineOptions.buildEnvironmentOptions : fallbackBuildEnvironmentOptions);
        setPipelineError([pipelines.error, pipelineOptions.error].filter(Boolean).join('；'));
        setVersionSource(workloads.config);
        setWorkloadSource(workloads.source);
        setWorkloadError(workloads.error || '');
        setVersionSourceMessage('');
        setFreightSerial(workloads.config.freightSerial);
      })
      .catch((error) => {
        if (cancelled) return;
        setTopology({
          ...deliveryTopology,
          applicationId: currentApplication.id,
          applicationName: currentApplication.name,
          topologyVersion: '后端加载失败',
          stages: [],
          edges: []
        });
        setFreightItems([]);
        setPublishGates([]);
        setDeploymentSource('api');
        setDeploymentError(error instanceof Error ? error.message : '部署数据加载失败');
        setActiveFreightId('');
        setRuntimeStageId(null);
        setConfigWorkloadId('');
        setSourcePipelines([]);
        setPipelineSource('api');
        setPipelineOptionSource('api');
        setRuntimeEnvironmentOptions([]);
        setBuildEnvironmentOptions([]);
        setPipelineError(error instanceof Error ? error.message : '流水线或环境数据加载失败');
        setVersionSource({
          updatedAt: '',
          freightSerial: initialVersionSourceConfig.freightSerial,
          workloads: []
        });
        setWorkloadSource('api');
        setWorkloadError(error instanceof Error ? error.message : '工作负载数据加载失败');
        setVersionSourceMessage('');
        setFreightSerial(initialVersionSourceConfig.freightSerial);
      })
      .finally(() => {
        if (!cancelled) {
          setDeploymentLoading(false);
          setPipelineLoading(false);
          setWorkloadLoading(false);
        }
      });
    setQuery('');
    setDraggingFreightId(null);
    setDragOverStageId(null);
    setConfirmTargetId(null);
    setConfigStageId(null);
    setVersionSourceOpen(false);
    setPipelineDialogOpen(false);
    setPipelineConfigId(null);
    setPipelineBuildId(null);
    setApprovalStageId(null);
    setPublishStageId(null);
    setHistoryStageId(null);
    setVerificationStageId(null);
    return () => {
      cancelled = true;
    };
  }, [currentApplication.id]);

  // WebSocket subscription for real-time updates
  useEffect(() => {
    paasWS.connect();
    paasWS.subscribe(currentApplication.id);

    const unsubMessage = paasWS.onMessage((msg: WSMessage) => {
      if (msg.app_id !== currentApplication.id) return;
      if (msg.type === 'stage_runtime_changed' || msg.type === 'build_status_changed' || msg.type === 'deployment_workspace_changed') {
        void reloadDeploymentPageData();
      }
    });

    const unsubReconnect = paasWS.onReconnect(() => {
      // Full refetch on reconnect
      void reloadDeploymentPageData();
    });

    return () => {
      unsubMessage();
      unsubReconnect();
      paasWS.unsubscribe(currentApplication.id);
    };
  }, [currentApplication.id]);

  const visibleFreights = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    const sorted = sortFreightsNewestFirst(freightItems);
    if (!keyword) return sorted;
    return sorted.filter((freight) => {
      const text = [
        freight.id,
        freight.commit,
        freight.source,
        ...freight.workloads.flatMap((workload) => [
          workload.name,
          workload.displayName,
          workload.pipeline,
          workload.version,
          workload.image,
          ...containersForFreightWorkload(workload).flatMap((container) => [
            container.name,
            container.pipeline,
            container.version,
            container.image,
            container.digest
          ])
        ])
      ].join(' ');
      return text.toLowerCase().includes(keyword);
    });
  }, [freightItems, query]);

  const hasDeliveryData = topology.stages.length > 0;
  const activeFreight = freightItems.find((freight) => freight.id === activeFreightId) || freightItems[0];
  const draggingFreight = freightItems.find((freight) => freight.id === draggingFreightId) || null;
  const confirmStage = topology.stages.find((stage) => stage.id === confirmTargetId);
  const runtimeStage = topology.stages.find((stage) => stage.id === runtimeStageId);
  const configStage = topology.stages.find((stage) => stage.id === configStageId);
  const approvalStage = topology.stages.find((stage) => stage.id === approvalStageId);
  const publishStage = topology.stages.find((stage) => stage.id === publishStageId);
  const historyStage = topology.stages.find((stage) => stage.id === historyStageId);
  const verificationStage = topology.stages.find((stage) => stage.id === verificationStageId);
  const configPipeline = sourcePipelines.find((pipeline) => pipeline.id === pipelineConfigId);
  const buildPipeline = sourcePipelines.find((pipeline) => pipeline.id === pipelineBuildId);
  const configWorkload = configStage?.workloads.find((workload) => workload.name === configWorkloadId) || configStage?.workloads[0];

  const currentVersionSourceFingerprint = useMemo(() => (
    versionSourceFingerprint(versionSource, sourcePipelines)
  ), [sourcePipelines, versionSource]);
  const versionSourceReadyForFreight = useMemo(() => (
    versionSourceHasRequiredSuccessfulVersions(versionSource, sourcePipelines)
  ), [sourcePipelines, versionSource]);
  const sourceDirty = useMemo(() => {
    if (!currentVersionSourceFingerprint || versionSource.workloads.length === 0) return false;
    if (!versionSourceReadyForFreight) return false;
    return !versionSourceConsumedByFreights(versionSource, sourcePipelines, freightItems, currentVersionSourceFingerprint);
  }, [currentVersionSourceFingerprint, freightItems, sourcePipelines, versionSource, versionSourceReadyForFreight]);

  useEffect(() => {
    if (pipelineSource !== 'api') return undefined;

    const handleFocus = () => {
      if (document.visibilityState === 'hidden') return;
      if (buildRefreshRef.current) return;
      buildRefreshRef.current = true;
      reloadPipelines().finally(() => { buildRefreshRef.current = false; });
    };
    window.addEventListener('focus', handleFocus);
    document.addEventListener('visibilitychange', handleFocus);

    return () => {
      window.removeEventListener('focus', handleFocus);
      document.removeEventListener('visibilitychange', handleFocus);
    };
  }, [pipelineSource, currentApplication.id]);

  function openConfig(stage: Stage) {
    setConfigStageId(stage.id);
    setConfigWorkloadId(stage.configurableWorkloadIds[0] || stage.workloads[0]?.name || '');
  }

  async function submitPromotion() {
    if (!confirmStage || !activeFreight) return;
    try {
      await createStagePromotion(currentApplication.id, confirmStage.id, activeFreight.id, promotionAutoPublish);
      setDeploymentError('');
    } catch (error) {
      setDeploymentError(error instanceof Error ? error.message : '创建 Promotion 失败');
    } finally {
      setPromotionAutoPublish(true);
      setConfirmTargetId(null);
      setDragOverStageId(null);
      setDraggingFreightId(null);
      void reloadDeploymentWorkspace();
    }
  }

  async function reloadDeploymentPageData(preferredFreightId?: string) {
    try {
      const { workspace, pipelines, pipelineOptions, workloads } = await loadDeploymentPageBundle(currentApplication.id);
      const sortedFreights = sortFreightsNewestFirst(workspace.freights);
      setTopology(workspace.topology);
      setFreightItems(sortedFreights);
      setPublishGates(workspace.publishGates || []);
      setDeploymentSource(workspace.source);
      setDeploymentError(workspace.error || '');
      setSourcePipelines((current) => mergePipelineRefresh(current, pipelines.pipelines));
      setPipelineSource(pipelines.source);
      setPipelineOptionSource(pipelineOptions.source);
      setRuntimeEnvironmentOptions(pipelineOptions.runtimeOptions.length ? pipelineOptions.runtimeOptions : fallbackRuntimeEnvironmentOptions);
      setBuildEnvironmentOptions(pipelineOptions.buildEnvironmentOptions.length ? pipelineOptions.buildEnvironmentOptions : fallbackBuildEnvironmentOptions);
      setPipelineError([pipelines.error, pipelineOptions.error].filter(Boolean).join('；'));
      setVersionSource(workloads.config);
      setWorkloadSource(workloads.source);
      setWorkloadError(workloads.error || '');
      setFreightSerial(workloads.config.freightSerial);
      setVerifiedStageKeys(verifiedKeysFromStages(workspace.topology.stages));
      setActiveFreightId((current) => {
        if (preferredFreightId && sortedFreights.some((freight) => freight.id === preferredFreightId)) return preferredFreightId;
        return sortedFreights.some((freight) => freight.id === current) ? current : sortedFreights[0]?.id || '';
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : '部署数据刷新失败';
      setDeploymentError(message);
      setPipelineError(message);
      setWorkloadError(message);
    }
  }

  async function reloadDeploymentWorkspace(preferredFreightId?: string) {
    try {
      const { workspace } = await loadDeploymentPageBundle(currentApplication.id);
      const sortedFreights = sortFreightsNewestFirst(workspace.freights);
      setTopology(workspace.topology);
      setFreightItems(sortedFreights);
      setPublishGates(workspace.publishGates || []);
      setDeploymentSource(workspace.source);
      setDeploymentError(workspace.error || '');
      setVerifiedStageKeys(verifiedKeysFromStages(workspace.topology.stages));
      setActiveFreightId((current) => {
        if (preferredFreightId && sortedFreights.some((freight) => freight.id === preferredFreightId)) return preferredFreightId;
        return sortedFreights.some((freight) => freight.id === current) ? current : sortedFreights[0]?.id || '';
      });
    } catch (error) {
      setTopology((current) => ({ ...current, stages: [], edges: [], topologyVersion: '后端加载失败' }));
      setFreightItems([]);
      setPublishGates([]);
      setDeploymentSource('api');
      setDeploymentError(error instanceof Error ? error.message : '部署数据加载失败');
      setActiveFreightId('');
    }
  }

function beginDrag(freight: Freight, event: DragEvent<HTMLElement>) {
  setActiveFreightId(freight.id);
  setDraggingFreightId(freight.id);
  event.dataTransfer.effectAllowed = 'copy';
  event.dataTransfer.setData('text/plain', freight.id);
  event.dataTransfer.setData('application/x-paas-freight-id', freight.id);

    const preview = document.createElement('div');
    const tone = toneForStage(deepestStageForFreight(freight));
    preview.className = 'drag-freight-preview';
    preview.style.setProperty('--drag-color', tone.hex);
    preview.style.setProperty('--drag-bg', 'white');
    preview.style.setProperty('--drag-fg', tone.fg);
    preview.innerHTML = `<strong>${freightDisplayName(freight)}</strong>`;
    document.body.appendChild(preview);
    event.dataTransfer.setDragImage(preview, 72, 28);
    window.setTimeout(() => preview.remove(), 0);
  }

  function finishDrag(freight: Freight, event: DragEvent<HTMLElement>) {
    const target = document
      .elementFromPoint(event.clientX, event.clientY)
      ?.closest<HTMLElement>('[data-stage-id]');
    const stage = topology.stages.find((item) => item.id === target?.dataset.stageId)
      || topology.stages.find((item) => item.id === dragOverStageId);
    if (stage && canDropFreight(stage, freight)) {
      dropFreight(stage, freight.id);
      return;
    }
    setDraggingFreightId(null);
    setDragOverStageId(null);
  }

  function canDropFreight(stage: Stage, freight: Freight | null) {
    if (!freight) return false;
    if (!stageHasBoundCluster(stage)) return false;
    return freight.eligibleStages.includes(stage.id);
  }

  function dropFreight(stage: Stage, freightId: string) {
    const freight = freightItems.find((item) => item.id === freightId);
    if (!canDropFreight(stage, freight || null)) {
      if (!stageHasBoundCluster(stage)) {
        setDeploymentError(`${stage.name} 未绑定集群，无法发布`);
      }
      setDragOverStageId(null);
      return;
    }
    setActiveFreightId(freightId);
    setConfirmTargetId(stage.id);
    setPromotionAutoPublish(true);
    setDragOverStageId(null);
  }

  async function refreshVersionSource() {
    const missingSuccessfulPipelines = versionSourceMissingSuccessfulPipelines(versionSource, sourcePipelines);
    if (missingSuccessfulPipelines.length > 0) {
      setVersionSourceMessage(`未发现可用成功版本：${missingSuccessfulPipelines.join('、')}`);
      return;
    }
    const nextFingerprint = currentVersionSourceFingerprint;
    if (!sourceDirty) {
      setVersionSourceMessage('未发现版本源变更，未生成新的 Freight');
      return;
    }
    if (deploymentSource === 'api') {
      try {
        setVersionSourceMessage('正在刷新版本源并创建发布包...');
        const name = compactFreightTimestamp(new Date());
        const freight = await createFreightFromVersionSource(currentApplication.id, name, versionSource, sourcePipelines, nextFingerprint);
        setVersionSourceMessage(`已生成发布包 ${freight?.name || freight?.id || name}`);
        await reloadDeploymentWorkspace(freight?.id);
      } catch (error) {
        setVersionSourceMessage(error instanceof Error ? error.message : '创建发布包失败');
      }
      return;
    }

    const nextSerial = freightSerial + 1;
    const nextFreight = nextFreightFromVersionSource(versionSource, sourcePipelines, nextSerial, nextFingerprint);
    setFreightSerial(nextSerial);
    setFreightItems((current) => [nextFreight, ...current]);
    setActiveFreightId(nextFreight.id);
    setVersionSourceMessage(`已生成发布包 ${freightDisplayName(nextFreight)}`);
  }

  async function saveVersionSource(nextConfig: VersionSourceConfig) {
    try {
      if (workloadSource === 'api') {
        const previousWorkloadIds = versionSource.workloads.map((w) => w.id);
        await saveVersionSourceWorkloads(currentApplication.id, nextConfig, previousWorkloadIds);
        await reloadDeploymentPageData();
      } else {
        setVersionSource(nextConfig);
      }
      setVersionSourceMessage('工作负载配置已保存，刷新版本源前会检查成功构建版本');
      setVersionSourceOpen(false);
      setWorkloadError('');
    } catch (error) {
      setWorkloadError(error instanceof Error ? error.message : '保存工作负载配置失败');
    }
  }

  async function reloadPipelines() {
    try {
      const bundle = await loadDeploymentPageBundle(currentApplication.id);
      setSourcePipelines((current) => mergePipelineRefresh(current, bundle.pipelines.pipelines));
      setPipelineSource(bundle.pipelines.source);
      setPipelineError(bundle.pipelines.error || '');
      setPipelineOptionSource(bundle.pipelineOptions.source);
      setRuntimeEnvironmentOptions(bundle.pipelineOptions.runtimeOptions.length ? bundle.pipelineOptions.runtimeOptions : fallbackRuntimeEnvironmentOptions);
      setBuildEnvironmentOptions(bundle.pipelineOptions.buildEnvironmentOptions.length ? bundle.pipelineOptions.buildEnvironmentOptions : fallbackBuildEnvironmentOptions);
      setVersionSource(bundle.workloads.config);
      setWorkloadSource(bundle.workloads.source);
      setWorkloadError(bundle.workloads.error || '');
      setFreightSerial(bundle.workloads.config.freightSerial);
    } catch (error) {
      setSourcePipelines([]);
      setPipelineSource('api');
      setPipelineError(error instanceof Error ? error.message : '流水线数据加载失败');
    }
  }

  async function createPipeline(input: VersionSourcePipeline) {
    try {
      const created = pipelineSource === 'api'
        ? await createVersionSourcePipeline(currentApplication.id, currentProject.id, input)
        : input;
      setSourcePipelines((current) => [created, ...current]);
      setVersionSourceMessage('流水线已添加，成功构建后才能生成新的 Freight');
      setPipelineDialogOpen(false);
      setPipelineError('');
      if (pipelineSource === 'api') void reloadPipelines();
    } catch (error) {
      setPipelineError(error instanceof Error ? error.message : '创建流水线失败');
    }
  }

  async function savePipelineConfig(input: VersionSourcePipeline) {
    try {
      const saved = pipelineSource === 'api'
        ? await updateVersionSourcePipeline(currentProject.id, input)
        : input;
      setSourcePipelines((current) => current.map((pipeline) => (
        pipeline.id === saved.id ? saved : pipeline
      )));
      setVersionSourceMessage('流水线配置已保存，成功构建后才能生成新的 Freight');
      setPipelineConfigId(null);
      setPipelineError('');
      if (pipelineSource === 'api') void reloadPipelines();
    } catch (error) {
      setPipelineError(error instanceof Error ? error.message : '保存流水线配置失败');
    }
  }

  async function deletePipeline(input: VersionSourcePipeline) {
    const referenced = versionSource.workloads.some((workload) => (
      workload.containers.some((container) => (
        container.imageSource.mode === 'pipeline' && container.imageSource.pipelineId === input.id
      ))
    ));
    if (referenced) return;
    try {
      if (pipelineSource === 'api') {
        await saveVersionSourceWorkloads(currentApplication.id, versionSource);
        await deleteVersionSourcePipeline(input.id);
      }
      setSourcePipelines((current) => current.filter((pipeline) => pipeline.id !== input.id));
      setPipelineConfigId((current) => (current === input.id ? null : current));
      setPipelineBuildId((current) => (current === input.id ? null : current));
      setVersionSourceMessage('流水线已删除，刷新版本源前会重新检查镜像来源');
      setPipelineError('');
    } catch (error) {
      setPipelineError(error instanceof Error ? error.message : '删除流水线失败');
    }
  }

  async function submitPipelineBuild(pipeline: VersionSourcePipeline, sourceRef: string, buildCommand: string, version: string) {
    if (pipelineSource === 'api') {
      try {
        const result = await triggerVersionSourcePipelineBuild(currentApplication.id, pipeline, sourceRef, buildCommand, version);
        const runningRun = normalizeTriggeredBuildRun(result.run);
        setSourcePipelines((current) => current.map((item) => (
          item.id === pipeline.id
            ? {
                ...item,
                branch: sourceRef || item.branch,
                buildCommand,
                sourcePath: item.sources[0]?.sourcePath || item.sourcePath,
                sources: item.sources.map((source, index) => index === 0 ? { ...source, branch: sourceRef || source.branch, buildCommand } : source),
                buildHistory: [
                  runningRun,
                  ...item.buildHistory.filter((run) => run.id !== runningRun.id)
                ],
                status: pipelineStatusFromBuildHistory([
                  runningRun,
                  ...item.buildHistory.filter((run) => run.id !== runningRun.id)
                ], item.status)
              }
            : item
        )));
        void reloadPipelines();
        setPipelineBuildId(pipeline.id);
        setPipelineError('');
      } catch (error) {
        setPipelineError(error instanceof Error ? error.message : '触发构建失败');
      }
      return;
    }
    setSourcePipelines((current) => current.map((item) => (
      item.id === pipeline.id
        ? {
            ...item,
            branch: sourceRef || item.branch,
            buildCommand,
            sourcePath: item.sources[0]?.sourcePath || item.sourcePath,
            sources: item.sources.map((source, index) => index === 0 ? { ...source, branch: sourceRef || source.branch, buildCommand } : source),
            buildHistory: [
              {
                id: `build-${item.id}-${Date.now().toString(36)}`,
                branch: sourceRef || item.branch,
                status: 'running',
                version,
                startedAt: '刚刚',
                duration: '进行中'
              },
              ...item.buildHistory
            ],
            status: 'running',
            logs: [
              `[刚刚] 拉取代码 ${item.sources[0]?.repository || '源码仓库'} ${sourceRef || item.branch}`,
              `[刚刚] 执行构建命令 ${buildCommand}`,
              '[刚刚] 构建任务已提交，等待平台回写镜像候选',
              ...item.logs
            ]
          }
        : item
    )));
    setPipelineBuildId(pipeline.id);
  }

  async function cancelPipelineBuild(pipeline: VersionSourcePipeline, buildRunId: string) {
    try {
      const cancelled = pipelineSource === 'api'
        ? await cancelVersionSourcePipelineBuild(buildRunId)
        : {
            id: buildRunId,
            branch: pipeline.branch,
            status: 'danger' as const,
            version: buildRunId,
            startedAt: '刚刚',
            duration: '已取消'
          };
      setSourcePipelines((current) => current.map((item) => (
        item.id === pipeline.id
          ? {
              ...item,
              buildHistory: item.buildHistory.map((run) => run.id === buildRunId ? cancelled : run),
              status: pipelineStatusFromBuildHistory(
                item.buildHistory.map((run) => run.id === buildRunId ? cancelled : run),
                item.status
              )
            }
          : item
      )));
      if (pipelineSource === 'api') void reloadPipelines();
      setPipelineError('');
    } catch (error) {
      setPipelineError(error instanceof Error ? error.message : '取消构建失败');
    }
  }

  return (
    <div className="divide-y">
      {!hasDeliveryData && (
        <Card>
          <CardContent className="flex min-h-[360px] flex-col items-center justify-center text-center">
            <CircleDashed className="h-10 w-10 text-muted-foreground" />
            <h2 className="mt-4 text-lg font-semibold">当前应用暂无交付流数据</h2>
            <p className="mt-2 max-w-xl text-sm text-muted-foreground">
              已切换到 {currentTenant.name} / {currentProject.name} / {currentApplication.name}。
              当前后端未返回可用 Stage DAG，或接口暂时不可用。
            </p>
            {deploymentError && <p className="mt-2 text-xs text-danger">{deploymentError}</p>}
          </CardContent>
        </Card>
      )}

      {hasDeliveryData && (
      <section className="grid gap-4 xl:min-h-[calc(100vh-220px)] xl:grid-cols-[348px_minmax(0,1fr)]">
        <div className="flex min-h-0 flex-col gap-4">
          <Card className="flex min-h-0 flex-1 flex-col xl:sticky xl:top-4 xl:max-h-[calc(100vh-112px)]">
            <CardHeader className="space-y-3">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <Archive className="h-5 w-5 text-primary" />
                  发布包
                </CardTitle>
              </div>
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input value={query} onChange={(event) => setQuery(event.target.value)} className="pl-9" placeholder="搜索发布包、Workload、镜像版本" />
              </div>
            </CardHeader>
            <CardContent className="min-h-0 flex-1 px-4 pb-4 pt-0">
              <div className="flex h-full min-h-0 flex-col items-center gap-3 overflow-y-auto pr-2">
                {visibleFreights.map((freight) => (
                  <FreightBundleCard
                    key={freight.id}
                    freight={freight}
                    stages={topology.stages}
                    active={freight.id === activeFreightId}
                    dragging={freight.id === draggingFreightId}
                    onSelect={() => setActiveFreightId(freight.id)}
                    onDragStart={(event) => beginDrag(freight, event)}
                    onDragEnd={(event) => finishDrag(freight, event)}
                  />
                ))}
              </div>
            </CardContent>
          </Card>

          {(deploymentError || pipelineError || workloadError || versionSourceMessage) && (
            <Card>
              <CardContent className="space-y-3 p-4">
              {deploymentError && (
                <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
                  {deploymentError}
                </div>
              )}
              {pipelineError && (
                <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
                  {pipelineError}
                </div>
              )}
              {workloadError && (
                <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
                  {workloadError}
                </div>
              )}
              {versionSourceMessage && (
                <div className="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900">
                  {versionSourceMessage}
                </div>
              )}
              </CardContent>
            </Card>
          )}
        </div>

        <Card className="flex min-h-[560px] flex-col xl:min-h-0">
          <CardHeader>
            <div className="flex flex-wrap justify-start gap-2">
              <Button variant="outline" size="sm" onClick={() => setPipelineDialogOpen(true)}>
                <Plus className="h-4 w-4" />
                创建流水线
              </Button>
              <Button variant="outline" size="sm" onClick={() => setShowPipelines((current) => !current)}>
                {showPipelines ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                {showPipelines ? '隐藏流水线' : '显示流水线'}
              </Button>
              {(deploymentLoading || pipelineLoading || workloadLoading) && <Badge variant="outline">加载中</Badge>}
            </div>
          </CardHeader>
          <CardContent className="min-h-0 flex-1">
            <TopologyBoard
              stages={topology.stages}
              edges={topology.edges}
              versionSource={versionSource}
              sourcePipelines={sourcePipelines}
              showPipelines={showPipelines}
              publishGates={publishGates}
              sourceDirty={sourceDirty}
              draggingFreight={draggingFreight}
              dragOverStageId={dragOverStageId}
              canDropFreight={canDropFreight}
              onDragEnterStage={setDragOverStageId}
              onDragLeaveStage={(stageId) => setDragOverStageId((current) => (current === stageId ? null : current))}
              onDropFreight={dropFreight}
              onOpenRuntime={(stage) => setRuntimeStageId(stage.id)}
              onOpenConfig={openConfig}
              onOpenVersionSource={() => setVersionSourceOpen(true)}
              onRefreshVersionSource={refreshVersionSource}
              onOpenPipelineConfig={(pipeline) => setPipelineConfigId(pipeline.id)}
              onOpenPipelineBuild={(pipeline) => setPipelineBuildId(pipeline.id)}
              onDeletePipeline={deletePipeline}
              onOpenPublish={setPublishStageId}
              onOpenHistory={(stage) => setHistoryStageId(stage.id)}
              onOpenConfigDiff={(stage) => setConfigDiffStage(stage)}
              onOpenVerification={(stage) => setVerificationStageId(stage.id)}
              verifiedStageKeys={verifiedStageKeys}
              freights={freightItems}
            />
          </CardContent>
        </Card>
      </section>
      )}

      {runtimeStage && (
        <RuntimeDrawer
          applicationId={currentApplication.id}
          stage={runtimeStage}
          freightDisplayName={freightItems.find((f) => f.id === runtimeStage.freightId)?.name || runtimeStage.freightId || '无'}
          onClose={() => setRuntimeStageId(null)}
          onReload={reloadDeploymentWorkspace}
        />
      )}
      {versionSourceOpen && (
        <VersionSourceConfigDialog
          config={versionSource}
          pipelines={sourcePipelines}
          onClose={() => setVersionSourceOpen(false)}
          onSave={saveVersionSource}
        />
      )}
      {pipelineDialogOpen && (
        <PipelineCreateDialog
          pipelines={sourcePipelines}
          runtimeOptions={runtimeEnvironmentOptions}
          buildEnvironmentOptions={buildEnvironmentOptions}
          onClose={() => setPipelineDialogOpen(false)}
          onCreate={createPipeline}
        />
      )}

      {configPipeline && (
        <PipelineConfigDialog
          pipeline={configPipeline}
          runtimeOptions={runtimeEnvironmentOptions}
          buildEnvironmentOptions={buildEnvironmentOptions}
          onClose={() => setPipelineConfigId(null)}
          onSave={savePipelineConfig}
        />
      )}

      {buildPipeline && (
        <PipelineBuildDialog
          pipeline={buildPipeline}
          onClose={() => setPipelineBuildId(null)}
          onSubmit={submitPipelineBuild}
          onCancelBuild={cancelPipelineBuild}
          onStreamLog={streamBuildRunLog}
          onBuildStatusChange={(buildRunId, status) => {
            setSourcePipelines((current) => current.map((item) => {
              if (!item.buildHistory.some((run) => run.id === buildRunId)) return item;
              const nextHistory = item.buildHistory.map((run) => (
                run.id === buildRunId ? { ...run, status } : run
              ));
              return {
                ...item,
                buildHistory: nextHistory,
                latestVersion: latestSuccessfulBuild({ ...item, buildHistory: nextHistory })?.version || '暂无版本',
                status: pipelineStatusFromBuildHistory(nextHistory, item.status)
              };
            }));
          }}
        />
      )}
      {configStage && (
        <StageConfigDialog
          applicationId={currentApplication.id}
          stage={configStage}
          workloads={versionSource.workloads}
          onClose={() => setConfigStageId(null)}
          onSaved={reloadDeploymentPageData}
        />
      )}
      {approvalStage && (
        <ApprovalReviewDialog
          applicationId={currentApplication.id}
          stage={approvalStage}
          onReviewed={() => {
            setDeploymentError('');
            void reloadDeploymentWorkspace();
          }}
          onError={(message) => setDeploymentError(message)}
          onClose={() => setApprovalStageId(null)}
        />
      )}
      {publishStage && (
        <PublishReviewDialog
          applicationId={currentApplication.id}
          stage={publishStage}
          onPublished={async () => {
            setDeploymentError('');
            await reloadDeploymentWorkspace();
          }}
          onError={(message) => setDeploymentError(message)}
          onClose={() => setPublishStageId(null)}
        />
      )}
      {historyStage && (
        <StageDeploymentHistoryDialog
          applicationId={currentApplication.id}
          stage={historyStage}
          onClose={() => setHistoryStageId(null)}
        />
      )}
      {verificationStage && (
        <VerificationDialog
          stage={verificationStage}
          freight={freightItems.find((freight) => freight.id === verificationStage.freightId) || null}
          onVerify={async (comment) => {
            try {
              if (!verificationStage.freightId) throw new Error('当前 Stage 没有已发布的 Freight，无法验证');
              await completeStageVerification(currentApplication.id, verificationStage.key, verificationStage.freightId, comment);
              setVerifiedStageKeys((current) => new Set(current).add(stageVerificationKey(verificationStage)));
              setDeploymentError('');
            } catch (error) {
              setDeploymentError(error instanceof Error ? error.message : '验证失败');
            } finally {
              setVerificationStageId(null);
              void reloadDeploymentWorkspace();
            }
          }}
          onClose={() => setVerificationStageId(null)}
        />
      )}
      {confirmStage && activeFreight && (
        <PromotionConfirmDialog
          freight={activeFreight}
          stage={confirmStage}
          topologyVersion={topology.topologyVersion}
          autoPublish={promotionAutoPublish}
          onAutoPublishChange={setPromotionAutoPublish}
          onClose={() => {
            setConfirmTargetId(null);
            setPromotionAutoPublish(true);
          }}
          onSubmit={submitPromotion}
        />
      )}
      {configDiffStage && (
        <ConfigDiffDialog
          applicationId={currentApplication.id}
          stage={configDiffStage}
          onClose={() => setConfigDiffStage(null)}
          onRedeployed={() => {
            setConfigDiffStage(null);
            void reloadDeploymentWorkspace();
          }}
        />
      )}
    </div>
  );
}

function FreightBundleCard({
  freight,
  stages,
  active,
  dragging,
  onSelect,
  onDragStart,
  onDragEnd
}: {
  freight: Freight;
  stages: Stage[];
  active: boolean;
  dragging: boolean;
  onSelect: () => void;
  onDragStart: (event: DragEvent<HTMLButtonElement>) => void;
  onDragEnd: (event: DragEvent<HTMLButtonElement>) => void;
}) {
  const currentStage = deepestStageForFreight(freight, stages);
  const tone = toneForStage(currentStage);
  const deployedStages = stagesByIds(freight.currentStages, stages);
  const eligibleStages = stagesByIds(freight.eligibleStages, stages);

  return (
    <button
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onClick={onSelect}
      style={toneStyle(tone)}
      className={cn(
        'relative flex w-[300px] shrink-0 flex-col overflow-hidden rounded-lg border border-[color:var(--stage-card-border)] border-l-[10px] border-l-[color:var(--stage-color)] bg-card text-left text-foreground shadow-sm transition-colors hover:border-[color:var(--stage-color)]',
        active && 'ring-2 ring-primary/20',
        dragging && 'opacity-45'
      )}
    >
      <div className="shrink-0 border-b bg-white px-4 py-3">
        <div className="flex items-start gap-3">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md border bg-background text-[color:var(--stage-color)]">
              <FileArchive className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <div
                className="mono inline-flex max-w-full rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-sm font-semibold text-amber-700"
                title={freightDisplayName(freight)}
              >
                <span className="truncate">{freightDisplayName(freight)}</span>
              </div>
              <div className="mono mt-1 truncate text-xs text-muted-foreground" title={freight.createdAt}>{freight.createdAt}</div>
            </div>
          </div>
        </div>
        <div className="mt-3 space-y-1 rounded-md border border-border/70 bg-muted/20 px-2.5 py-2">
          <FreightStageSummaryRow label="可到" stages={eligibleStages} emptyText="暂无" />
          <FreightStageSummaryRow label="已到" stages={deployedStages} emptyText="无" />
        </div>
      </div>
      <div className="divide-y">
        {freight.workloads.map((workload) => (
          <FreightWorkloadRow key={workload.name} workload={workload} />
        ))}
      </div>
    </button>
  );
}

function FreightStageSummaryRow({ label, stages, emptyText }: { label: string; stages: Stage[]; emptyText: string }) {
  return (
    <div className="grid grid-cols-[44px_minmax(0,1fr)] items-start gap-2 text-xs leading-5">
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <div className="min-w-0 text-foreground">
        {stages.length ? (
          <span className="inline-flex flex-wrap items-center gap-x-1 gap-y-0.5">
            {stages.map((stage, index) => {
              const color = stagePalette[inferStageColorToken(stage)].hex;
              return (
                <span key={stage.id} className="inline-flex items-center gap-x-1">
                  {index > 0 && <span className="text-muted-foreground/50">|</span>}
                  <span className="font-medium" style={{ color }}>{stage.name}</span>
                </span>
              );
            })}
          </span>
        ) : (
          <span className="text-muted-foreground">{emptyText}</span>
        )}
      </div>
    </div>
  );
}

function FreightWorkloadRow({ workload }: { workload: Freight['workloads'][number] }) {
  const containers = containersForFreightWorkload(workload);
  return (
    <div className="px-4 py-3">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-medium" title={workload.displayName}>{workload.displayName}</div>
        </div>
        <Badge variant="outline" className="shrink-0 whitespace-nowrap">{containers.length} 个镜像</Badge>
      </div>
      <div className="mt-2 space-y-1.5">
        {containers.map((container) => (
          <div key={container.name} className="rounded-md border bg-slate-50 px-2.5 py-2 text-xs">
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <div className="truncate font-medium" title={container.name}>{container.name}</div>
              </div>
              <div className="mono shrink-0 font-semibold" title={freightContainerVersionLabel(container)}>
                {freightContainerVersionLabel(container)}
              </div>
            </div>
            {container.sourceMode === 'custom' && (
              <div className="mono mt-1 truncate text-muted-foreground" title={freightContainerImageLabel(container)}>
                {freightContainerImageLabel(container)}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function freightContainerVersionLabel(container: DeploymentFreightContainer) {
  return container.sourceMode === 'custom' ? '自定义' : (container.version || '-');
}

function freightContainerImageLabel(container: DeploymentFreightContainer) {
  if (!container.image || container.image === '-') return '-';
  return container.image.split('@')[0];
}

function containersForFreightWorkload(workload: Freight['workloads'][number]): DeploymentFreightContainer[] {
  if (workload.containers?.length) return workload.containers;
  return [{
    name: 'app',
    pipeline: workload.pipeline,
    version: workload.version,
    image: workload.image,
    digest: workload.digest,
    status: workload.status
  }];
}

function workloadKindLabel(kind?: string) {
  return String(kind || '').toLowerCase() === 'statefulset' ? '有状态' : '无状态';
}

function TopologyBoard({
  stages,
  edges,
  versionSource,
  sourcePipelines,
  showPipelines,
  publishGates,
  sourceDirty,
  draggingFreight,
  dragOverStageId,
  canDropFreight,
  onDragEnterStage,
  onDragLeaveStage,
  onDropFreight,
  onOpenRuntime,
  onOpenConfig,
  onOpenVersionSource,
  onRefreshVersionSource,
  onOpenPipelineConfig,
  onOpenPipelineBuild,
  onDeletePipeline,
  onOpenPublish,
  onOpenHistory,
  onOpenConfigDiff,
  onOpenVerification,
  verifiedStageKeys,
  freights
}: {
  stages: Stage[];
  edges: DeploymentEdge[];
  versionSource: VersionSourceConfig;
  sourcePipelines: VersionSourcePipeline[];
  showPipelines: boolean;
  publishGates: PublishGateSummary[];
  sourceDirty: boolean;
  draggingFreight: Freight | null;
  dragOverStageId: string | null;
  canDropFreight: (stage: Stage, freight: Freight | null) => boolean;
  onDragEnterStage: (stageId: string) => void;
  onDragLeaveStage: (stageId: string) => void;
  onDropFreight: (stage: Stage, freightId: string) => void;
  onOpenRuntime: (stage: Stage) => void;
  onOpenConfig: (stage: Stage) => void;
  onOpenVersionSource: () => void;
  onRefreshVersionSource: () => void;
  onOpenPipelineConfig: (pipeline: VersionSourcePipeline) => void;
  onOpenPipelineBuild: (pipeline: VersionSourcePipeline) => void;
  onDeletePipeline: (pipeline: VersionSourcePipeline) => void;
  onOpenPublish: (stageId: string) => void;
  onOpenHistory: (stage: Stage) => void;
  onOpenConfigDiff: (stage: Stage) => void;
  onOpenVerification: (stage: Stage) => void;
  verifiedStageKeys: Set<string>;
  freights: Freight[];
}) {
  const firstStageColumnCenterY = useMemo(() => firstStageColumnCenter(stages), [stages]);

  const linkedPipelineIds = useMemo(() => {
    const ids = new Set<string>();
    versionSource.workloads.forEach((workload) => {
      workload.containers.forEach((container) => {
        if (container.imageSource.mode === 'pipeline' && container.imageSource.pipelineId) {
          ids.add(container.imageSource.pipelineId);
        }
      });
    });
    return ids;
  }, [versionSource]);

  const nodes = useMemo<(StageFlowNode | WarehouseFlowNode | PipelineFlowNode)[]>(() => {
    const pipelineGroupTop = centeredStackTop(firstStageColumnCenterY, sourcePipelines.length, PIPELINE_NODE_SPACING, FLOW_CARD_ESTIMATED_HEIGHT);
    const pipelineNodes: PipelineFlowNode[] = sourcePipelines.map((pipeline, index) => ({
      id: `pipeline-${pipeline.id}`,
      type: 'pipelineNode',
      position: { x: 0, y: pipelineGroupTop + index * PIPELINE_NODE_SPACING },
      draggable: false,
      selectable: false,
      data: {
        pipeline,
        referenced: linkedPipelineIds.has(pipeline.id),
        onOpenConfig: onOpenPipelineConfig,
        onOpenBuild: onOpenPipelineBuild,
        onDelete: onDeletePipeline
      }
    }));

    const warehouseNode: WarehouseFlowNode = {
      id: WAREHOUSE_NODE_ID,
      type: 'warehouseNode',
      position: { x: FLOW_COLUMN_WIDTH, y: firstStageColumnCenterY - FLOW_CARD_ESTIMATED_HEIGHT / 2 },
      draggable: false,
      selectable: false,
      data: {
        config: versionSource,
        dirty: sourceDirty,
        onRefresh: onRefreshVersionSource,
        onOpenConfig: onOpenVersionSource
      }
    };

    const stageNodes: StageFlowNode[] = stages.map((stage) => {
      const publishGate = publishGates.find((item) => item.targetStageKey === stage.key || item.targetStageKey === stage.id);
      const freight = freights.find((item) => item.id === stage.freightId) || null;

      return {
        id: stage.id,
        type: 'stageNode',
        position: stageNodePosition(stage, showPipelines ? 2 : 0),
        draggable: false,
        selectable: false,
        data: {
          stage,
          freight,
          freightDisplayName: freight?.name || stage.freightId || '无',
          dragging: Boolean(draggingFreight),
          canDrop: canDropFreight(stage, draggingFreight),
          dragOver: dragOverStageId === stage.id,
          verificationDone: verifiedStageKeys.has(stageVerificationKey(stage)),
          canVerify: stageRequiresVerification(stage),
          publishPendingCount: publishGate?.pendingCount || 0,
          canPublishPending: publishGate ? publishGate.canPublish : false,
          onDragEnterStage,
          onDragLeaveStage,
          onDropFreight,
          onOpenRuntime,
          onOpenConfig,
          onOpenHistory,
          onOpenVerification,
          onOpenPublish,
          onOpenConfigDiff
        }
      };
    });

    return showPipelines ? [...pipelineNodes, warehouseNode, ...stageNodes] : stageNodes;
  }, [
    sourcePipelines,
    firstStageColumnCenterY,
    linkedPipelineIds,
    showPipelines,
    versionSource,
    sourceDirty,
    onRefreshVersionSource,
    onOpenVersionSource,
    onOpenPipelineConfig,
    onOpenPipelineBuild,
    onDeletePipeline,
    stages,
    draggingFreight,
    dragOverStageId,
    canDropFreight,
    verifiedStageKeys,
    onDragEnterStage,
    onDragLeaveStage,
    onDropFreight,
    onOpenRuntime,
    onOpenConfig,
    onOpenHistory,
    onOpenVerification,
    onOpenPublish,
    onOpenConfigDiff,
    publishGates,
    freights
  ]);

  const flowEdges = useMemo<Edge[]>(() => {
    const pipelineEdges: Edge[] = showPipelines ? sourcePipelines
      .filter((pipeline) => linkedPipelineIds.has(pipeline.id))
      .map((pipeline) => ({
        id: `pipeline-${pipeline.id}-${WAREHOUSE_NODE_ID}`,
        source: `pipeline-${pipeline.id}`,
        target: WAREHOUSE_NODE_ID,
        sourceHandle: 'right',
        targetHandle: 'left',
        type: 'default',
        markerEnd: { type: MarkerType.ArrowClosed, color: '#0072B2' },
        style: { stroke: '#0072B2', strokeWidth: 1.8, strokeDasharray: '4 4' }
      })) : [];

    const entryStages = entryStagesForVersionSource(stages, edges);
    const warehouseEdges: Edge[] = showPipelines ? entryStages.map((stage) => ({
      id: `${WAREHOUSE_NODE_ID}-${stage.id}`,
      source: WAREHOUSE_NODE_ID,
      target: stage.id,
      sourceHandle: 'right',
      targetHandle: 'left',
      type: 'default',
      markerEnd: { type: MarkerType.ArrowClosed, color: '#64748B' },
      style: { stroke: '#64748B', strokeWidth: 1.8 }
    })) : [];

    const stageEdges: ApprovalEdge[] = edges.flatMap((edge) => {
      const sourceStage = stages.find((stage) => stage.id === edge.fromStageId);
      const targetStage = stages.find((stage) => stage.id === edge.toStageId);
      if (!sourceStage || !targetStage) return [];
      const handles = edgeHandles(sourceStage, targetStage);
      const tone = toneForStage(sourceStage);
      return [{
        id: `${edge.fromStageId}-${edge.toStageId}`,
        type: 'approvalEdge',
        source: edge.fromStageId,
        target: edge.toStageId,
        sourceHandle: handles.sourceHandle,
        targetHandle: handles.targetHandle,
        markerEnd: { type: MarkerType.ArrowClosed, color: tone.hex },
        style: { stroke: tone.hex, strokeWidth: 2 },
        data: {}
      }];
    });

    return [...pipelineEdges, ...warehouseEdges, ...stageEdges];
  }, [sourcePipelines, linkedPipelineIds, showPipelines, stages, edges]);

  function handleCanvasDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    const freightId = freightIdFromDrag(event);
    const target = document
      .elementFromPoint(event.clientX, event.clientY)
      ?.closest<HTMLElement>('[data-stage-id]');
    const stage = stages.find((item) => item.id === target?.dataset.stageId);
    if (stage && freightId) {
      onDropFreight(stage, freightId);
    }
  }

  return (
    <div className="relative flex h-full min-h-[520px] overflow-hidden rounded-lg border bg-slate-50 p-4">
      <div className="min-h-0 flex-1 overflow-hidden rounded-md bg-slate-50">
        <ReactFlow
          nodes={nodes}
          edges={flowEdges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          fitView
          fitViewOptions={{ padding: 0.22, minZoom: 0.58, maxZoom: 1.18 }}
          minZoom={0.55}
          maxZoom={1.8}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={false}
          panOnScroll={false}
          panOnDrag
          zoomOnScroll
          zoomOnPinch
          zoomOnDoubleClick
          onDragOver={(event) => {
            event.preventDefault();
            event.dataTransfer.dropEffect = draggingFreight ? 'copy' : 'none';
          }}
          onDrop={handleCanvasDrop}
          proOptions={{ hideAttribution: true }}
          className="deployment-flow"
        >
          <Background color="rgba(148,163,184,0.38)" gap={40} />
          <Controls showInteractive />
        </ReactFlow>
      </div>
    </div>
  );
}

function stageNodePosition(stage: Stage, leadingColumns = 2) {
  const col = stage.col ?? stage.lane ?? 0;
  const row = stage.row ?? 0;
  return {
    x: (col + leadingColumns) * FLOW_COLUMN_WIDTH,
    y: row * FLOW_ROW_HEIGHT
  };
}

function firstStageColumnCenter(stages: Stage[]) {
  if (!stages.length) return FLOW_CARD_ESTIMATED_HEIGHT / 2;
  const minCol = Math.min(...stages.map((stage) => stage.col ?? stage.lane ?? 0));
  const rows = stages
    .filter((stage) => (stage.col ?? stage.lane ?? 0) === minCol)
    .map((stage) => stage.row ?? 0);
  if (!rows.length) return FLOW_CARD_ESTIMATED_HEIGHT / 2;
  const minRow = Math.min(...rows);
  const maxRow = Math.max(...rows);
  return ((minRow + maxRow) / 2) * FLOW_ROW_HEIGHT + FLOW_CARD_ESTIMATED_HEIGHT / 2;
}

function entryStagesForVersionSource(stages: Stage[], edges: DeploymentEdge[]) {
  if (!stages.length) return [];

  const columns = stages.map((stage) => stage.col ?? stage.lane ?? 0);
  if (columns.every((column) => Number.isFinite(column))) {
    const minCol = Math.min(...columns);
    return stages
      .filter((stage) => (stage.col ?? stage.lane ?? 0) === minCol)
      .sort((a, b) => (a.row ?? 0) - (b.row ?? 0));
  }

  const stageIds = new Set(stages.map((stage) => stage.id));
  const incomingStageIds = new Set(
    edges
      .map((edge) => edge.toStageId)
      .filter((stageId) => stageIds.has(stageId))
  );
  const entries = stages.filter((stage) => !incomingStageIds.has(stage.id));
  return entries.length ? entries : [stages[0]];
}

function centeredStackTop(centerY: number, itemCount: number, spacing: number, itemHeight: number) {
  if (itemCount <= 0) return centerY - itemHeight / 2;
  const totalHeight = (itemCount - 1) * spacing + itemHeight;
  return centerY - totalHeight / 2;
}

function edgeHandles(source: Stage, target: Stage) {
  const sourceCol = source.col ?? source.lane ?? 0;
  const targetCol = target.col ?? target.lane ?? 0;
  const sourceRow = source.row ?? 0;
  const targetRow = target.row ?? 0;

  if (sourceCol === targetCol) {
    return targetRow >= sourceRow
      ? { sourceHandle: 'bottom', targetHandle: 'top' }
      : { sourceHandle: 'top', targetHandle: 'bottom' };
  }

  return targetCol >= sourceCol
    ? { sourceHandle: 'right', targetHandle: 'left' }
    : { sourceHandle: 'left', targetHandle: 'right' };
}

function ApprovalGateEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  markerEnd,
  style
}: EdgeProps<ApprovalEdge>) {
  const [edgePath] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition
  });

  return (
    <BaseEdge id={id} path={edgePath} markerEnd={markerEnd} style={style} />
  );
}

function WarehouseNodeCard({ data }: NodeProps<WarehouseFlowNode>) {
  const containerCount = data.config.workloads.reduce((total, workload) => total + workload.containers.length, 0);
  const pipelineLinkedCount = data.config.workloads.reduce((total, workload) => (
    total + workload.containers.filter((container) => container.imageSource.mode === 'pipeline').length
  ), 0);

  return (
    <div
      className="pointer-events-auto nodrag nopan nowheel flex flex-col overflow-hidden rounded-lg border bg-card text-left shadow-sm"
      style={{ width: FLOW_CARD_WIDTH, minHeight: 220 }}
    >
      <Handle id="left" type="target" position={Position.Left} className="!h-2 !w-2 !border-0 !bg-transparent" />
      <Handle id="right" type="source" position={Position.Right} className="!h-2 !w-2 !border-0 !bg-transparent" />
      <div className="border-b bg-slate-900 px-4 py-3 text-white">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.12em] text-white/70">
              <PackageOpen className="h-4 w-4" />
              版本源
            </div>
            <div className="mt-1 text-base font-semibold">部署配置</div>
          </div>
          {data.dirty && <span className="rounded bg-amber-400 px-2 py-0.5 text-[11px] font-semibold text-slate-950">有变更</span>}
        </div>
      </div>
      <div className="flex min-h-0 flex-1 flex-col space-y-3 p-4">
        <div className="grid grid-cols-3 gap-2 text-center text-xs">
          <div className="rounded-md border bg-slate-50 p-2">
            <div className="mono text-sm font-semibold text-foreground">{data.config.workloads.length}</div>
            <div className="mt-0.5 text-muted-foreground">Workload</div>
          </div>
          <div className="rounded-md border bg-slate-50 p-2">
            <div className="mono text-sm font-semibold text-foreground">{containerCount}</div>
            <div className="mt-0.5 text-muted-foreground">容器</div>
          </div>
          <div className="rounded-md border bg-slate-50 p-2">
            <div className="mono text-sm font-semibold text-foreground">{pipelineLinkedCount}</div>
            <div className="mt-0.5 text-muted-foreground">流水线镜像</div>
          </div>
        </div>
        <div className="mt-auto flex gap-2">
          <button
            type="button"
            className="nodrag nopan nowheel inline-flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md border bg-card px-2.5 text-xs font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
            onPointerDownCapture={(event) => event.stopPropagation()}
            onMouseDownCapture={(event) => event.stopPropagation()}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              data.onRefresh();
            }}
          >
            <RefreshCw className="h-3.5 w-3.5" />
            刷新
          </button>
          <button
            type="button"
            className="nodrag nopan nowheel inline-flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md border bg-card px-2.5 text-xs font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
            onPointerDownCapture={(event) => event.stopPropagation()}
            onMouseDownCapture={(event) => event.stopPropagation()}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              data.onOpenConfig();
            }}
          >
            <Settings2 className="h-3.5 w-3.5" />
            配置
          </button>
        </div>
      </div>
    </div>
  );
}

function PipelineNodeCard({ data }: NodeProps<PipelineFlowNode>) {
  return (
    <div className="pointer-events-auto nodrag nopan nowheel w-[300px] overflow-hidden rounded-lg border bg-card text-left shadow-sm">
      <Handle id="right" type="source" position={Position.Right} className="!h-2 !w-2 !border-0 !bg-transparent" />
      <div className="border-l-[8px] border-l-[#0072B2] p-3">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-xs font-semibold text-muted-foreground">
              <Workflow className="h-4 w-4 text-primary" />
              流水线
            </div>
            <div className="mt-1 truncate text-sm font-semibold">{data.pipeline.name}</div>
          </div>
          <BuildRunStatusBadge status={data.pipeline.status} />
        </div>
        <div className="mt-3 grid grid-cols-[72px_minmax(0,1fr)] gap-y-1 text-xs">
          <span className="text-muted-foreground">分支</span>
          <span className="mono truncate">{data.pipeline.branch}</span>
          <span className="text-muted-foreground">运行时</span>
          <span className="truncate">{data.pipeline.runtime}</span>
          <span className="text-muted-foreground">最新版本</span>
          <span className="mono truncate font-medium">{data.pipeline.latestVersion}</span>
        </div>
        <div className="mt-3 grid grid-cols-3 gap-2">
          <button
            type="button"
            className="nodrag nopan nowheel inline-flex h-8 items-center justify-center gap-1.5 rounded-md border bg-card px-2.5 text-xs font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
            onPointerDownCapture={(event) => event.stopPropagation()}
            onMouseDownCapture={(event) => event.stopPropagation()}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              data.onOpenConfig(data.pipeline);
            }}
          >
            <Settings2 className="h-3.5 w-3.5" />
            配置
          </button>
          <button
            type="button"
            title="打开构建窗口"
            className="nodrag nopan nowheel inline-flex h-8 items-center justify-center gap-1.5 rounded-md border bg-card px-2.5 text-xs font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
            onPointerDownCapture={(event) => event.stopPropagation()}
            onMouseDownCapture={(event) => event.stopPropagation()}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              data.onOpenBuild(data.pipeline);
            }}
          >
            <Rocket className="h-3.5 w-3.5" />
            构建
          </button>
          <button
            type="button"
            disabled={data.referenced}
            title={data.referenced ? '该流水线已被版本源引用，不能删除' : '删除流水线'}
            className={cn(
              'nodrag nopan nowheel inline-flex h-8 items-center justify-center gap-1.5 rounded-md border bg-card px-2.5 text-xs font-medium transition-colors hover:bg-red-50 hover:text-red-700',
              data.referenced && 'cursor-not-allowed opacity-45 hover:bg-card hover:text-foreground'
            )}
            onPointerDownCapture={(event) => event.stopPropagation()}
            onMouseDownCapture={(event) => event.stopPropagation()}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              if (!data.referenced) data.onDelete(data.pipeline);
            }}
          >
            <Trash2 className="h-3.5 w-3.5" />
            删除
          </button>
        </div>
      </div>
    </div>
  );
}

function StageFlowNodeCard({ data }: NodeProps<StageFlowNode>) {
  return (
    <div
      className="pointer-events-auto nodrag nopan nowheel"
      data-stage-id={data.stage.id}
    >
      <Handle id="left" type="target" position={Position.Left} className="!h-2 !w-2 !border-0 !bg-transparent" />
      <Handle id="right" type="source" position={Position.Right} className="!h-2 !w-2 !border-0 !bg-transparent" />
      <Handle id="top" type="target" position={Position.Top} className="!h-2 !w-2 !border-0 !bg-transparent" />
      <Handle id="bottom" type="source" position={Position.Bottom} className="!h-2 !w-2 !border-0 !bg-transparent" />
      <StageCard
        stage={data.stage}
        freight={data.freight}
        freightDisplayName={data.freightDisplayName}
        dragging={data.dragging}
        canDrop={data.canDrop}
        dragOver={data.dragOver}
        verificationDone={data.verificationDone}
        onDragEnter={() => data.onDragEnterStage(data.stage.id)}
        onDragLeave={() => data.onDragLeaveStage(data.stage.id)}
        onDropFreight={(freightId) => data.onDropFreight(data.stage, freightId)}
        onOpenRuntime={() => data.onOpenRuntime(data.stage)}
        onOpenConfig={() => data.onOpenConfig(data.stage)}
        onOpenHistory={() => data.onOpenHistory(data.stage)}
        onOpenVerification={() => data.onOpenVerification(data.stage)}
        canVerify={data.canVerify}
        publishPendingCount={data.publishPendingCount}
        canPublishPending={data.canPublishPending}
        onOpenPublish={() => data.onOpenPublish(data.stage.id)}
        onOpenConfigDiff={() => data.onOpenConfigDiff(data.stage)}
      />
    </div>
  );
}

function StageFreightTooltip({
  freight,
  freightDisplayName
}: {
  freight: Freight;
  freightDisplayName: string;
}) {
  return (
    <div
      className="group relative inline-flex min-w-0 justify-end"
      onPointerDownCapture={(event) => event.stopPropagation()}
      onMouseDownCapture={(event) => event.stopPropagation()}
      onClick={(event) => event.stopPropagation()}
    >
      <div
        tabIndex={0}
        className="nodrag nopan nowheel mono max-w-[168px] cursor-default truncate rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-right font-semibold text-amber-700 transition-colors hover:bg-amber-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-200"
        aria-label={`查看发布包 ${freightDisplayName} 的版本明细`}
      >
        {freightDisplayName}
      </div>
      <div className="pointer-events-none absolute right-0 top-full z-50 mt-2 hidden w-[320px] rounded-lg border border-slate-200 bg-white p-3 text-left text-xs text-slate-700 shadow-xl group-hover:block group-focus-within:block">
        <div className="mb-2 border-b border-slate-100 pb-2">
          <div className="text-[11px] font-medium text-slate-500">发布包明细</div>
          <div className="mono mt-0.5 truncate font-semibold text-slate-800">{freightDisplayName}</div>
        </div>
        <div className="max-h-64 space-y-2 overflow-auto pr-1">
          {freight.workloads.map((workload) => {
            const containers = containersForFreightWorkload(workload);
            return (
              <div key={workload.name} className="rounded-md border border-slate-100 bg-slate-50/70 p-2">
                <div className="mb-1.5 truncate font-medium text-slate-800" title={workload.displayName}>
                  {workload.displayName}
                </div>
                <div className="space-y-1.5">
                  {containers.map((container) => {
                    return (
                      <div key={`${workload.name}-${container.name}`} className="rounded border border-slate-200 bg-white px-2 py-1.5">
                        <div className="flex items-center justify-between gap-2">
                          <span className="min-w-0 truncate text-slate-600" title={container.name}>{container.name}</span>
                          <span className="mono shrink-0 font-semibold text-slate-800" title={freightContainerVersionLabel(container)}>
                            {freightContainerVersionLabel(container)}
                          </span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function normalizeStageHealth(stage: Stage): ArgoHealthStatus {
  const value = String((stage as Stage & { healthStatus?: string }).healthStatus || '').toLowerCase();
  if (value.includes('healthy') || value.includes('健康')) return 'Healthy';
  if (value.includes('progress') || value.includes('进行中')) return 'Progressing';
  if (value.includes('degrad') || value.includes('降级')) return 'Degraded';
  if (value.includes('suspend') || value.includes('暂停')) return 'Suspended';
  if (value.includes('missing') || value.includes('缺失')) return 'Missing';
  if (stage.status === 'healthy') return 'Healthy';
  if (stage.status === 'running') return 'Progressing';
  if (stage.status === 'danger') return 'Degraded';
  return 'Unknown';
}

function normalizeStageSync(stage: Stage): ArgoSyncStatus {
  const value = String((stage as Stage & { syncStatus?: string }).syncStatus || stage.sync || '').toLowerCase();
  if (value === 'synced' || value.includes('已同步')) return 'Synced';
  if (value === 'outofsync' || value.includes('out of sync') || value.includes('未同步') || value.includes('不同步')) return 'OutOfSync';
  return 'Unknown';
}

function argoHealthMeta(status: ArgoHealthStatus) {
  switch (status) {
    case 'Healthy':
      return { label: '健康', rawLabel: 'Healthy', Icon: CheckCircle2, className: 'border-emerald-200 bg-emerald-50 text-emerald-700' };
    case 'Progressing':
      return { label: '进行中', rawLabel: 'Progressing', Icon: CircleDashed, className: 'border-sky-200 bg-sky-50 text-sky-700' };
    case 'Degraded':
      return { label: '已降级', rawLabel: 'Degraded', Icon: X, className: 'border-rose-200 bg-rose-50 text-rose-700' };
    case 'Suspended':
      return { label: '已暂停', rawLabel: 'Suspended', Icon: PauseCircle, className: 'border-amber-200 bg-amber-50 text-amber-700' };
    case 'Missing':
      return { label: '缺失', rawLabel: 'Missing', Icon: PackageOpen, className: 'border-slate-300 bg-slate-100 text-slate-700' };
    default:
      return { label: '未知', rawLabel: 'Unknown', Icon: CircleDashed, className: 'border-slate-300 bg-slate-50 text-slate-600' };
  }
}

function argoSyncMeta(status: ArgoSyncStatus) {
  switch (status) {
    case 'Synced':
      return { label: '已同步', rawLabel: 'Synced', Icon: CheckCircle2, className: 'border-emerald-200 bg-emerald-50 text-emerald-700' };
    case 'OutOfSync':
      return { label: '未同步', rawLabel: 'OutOfSync', Icon: GitPullRequestArrow, className: 'border-amber-200 bg-amber-50 text-amber-700' };
    default:
      return { label: '未知', rawLabel: 'Unknown', Icon: CircleDashed, className: 'border-slate-300 bg-slate-50 text-slate-600' };
  }
}

function StageCard({
  stage,
  freight,
  freightDisplayName,
  dragging,
  canDrop,
  dragOver,
  onDragEnter,
  onDragLeave,
  onDropFreight,
  onOpenRuntime,
  onOpenConfig,
  onOpenHistory,
  onOpenVerification,
  canVerify,
  publishPendingCount,
  canPublishPending,
  onOpenPublish,
  onOpenConfigDiff,
  verificationDone
}: {
  stage: Stage;
  freight: Freight | null;
  freightDisplayName: string;
  dragging: boolean;
  canDrop: boolean;
  dragOver: boolean;
  verificationDone: boolean;
  onDragEnter: () => void;
  onDragLeave: () => void;
  onDropFreight: (freightId: string) => void;
  onOpenRuntime: () => void;
  onOpenConfig: () => void;
  onOpenHistory: () => void;
  onOpenVerification: () => void;
  canVerify: boolean;
  publishPendingCount: number;
  canPublishPending: boolean;
  onOpenPublish: () => void;
  onOpenConfigDiff: () => void;
}) {
  const tone = toneForStage(stage);
  const needsVerification = stageRequiresVerification(stage);
  const hasBoundCluster = stageHasBoundCluster(stage);
  const healthMeta = argoHealthMeta(normalizeStageHealth(stage));
  const syncMeta = argoSyncMeta(normalizeStageSync(stage));
  const HealthIcon = healthMeta.Icon;
  const SyncIcon = syncMeta.Icon;
  const verificationPending = needsVerification && Boolean(stage.freightId) && !verificationDone;

  return (
    <div
      data-stage-id={stage.id}
      onDragEnter={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onDragEnter();
      }}
      onDragOver={(event) => {
        event.preventDefault();
        event.stopPropagation();
        event.dataTransfer.dropEffect = canDrop ? 'copy' : 'none';
      }}
      onDragLeave={(event) => {
        event.stopPropagation();
        onDragLeave();
      }}
      onDrop={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onDropFreight(freightIdFromDrag(event));
      }}
      className={cn(
        'nodrag nopan flex flex-col overflow-visible rounded-lg border border-[color:var(--stage-card-border)] bg-card text-left text-foreground shadow-sm transition-all hover:border-[color:var(--stage-color)]',
        dragging && canDrop && 'border-[color:var(--stage-color)] ring-2 ring-primary/20',
        dragging && !canDrop && 'opacity-40 grayscale',
        dragOver && canDrop && 'scale-[1.02] shadow-lg',
        dragOver && !canDrop && 'ring-2 ring-slate-300'
      )}
      style={{ ...toneStyle(tone), width: FLOW_CARD_WIDTH, minHeight: 260 }}
    >
      <div className="flex min-h-11 items-center justify-between gap-2 rounded-t-lg bg-[color:var(--stage-color)] px-3 py-2.5 text-white">
        <div className="min-w-0">
          <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-white/75">{stage.key}</div>
          <div className="truncate text-base font-semibold leading-tight">{stage.name}</div>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          <StageHeaderAction
            label={verificationDone ? "已准出" : "准出"}
            icon={<ShieldCheck className="h-3.5 w-3.5" />}
            badge={needsVerification && verificationPending ? 1 : 0}
            ariaLabel={`准出${stage.name}当前发布包`}
            onClick={onOpenVerification}
            disabled={!canVerify || verificationDone}
            transparentWhenDisabled
          />
          <StageHeaderAction
            label="发布"
            icon={<Rocket className="h-3.5 w-3.5" />}
            badge={canPublishPending ? publishPendingCount : 0}
            ariaLabel={`发布待发布到${stage.name}的发布包`}
            onClick={onOpenPublish}
          />
        </div>
      </div>
      <div className="mx-3 mt-3 space-y-2 rounded-md border border-dashed border-[color:var(--stage-chip-border)] bg-background p-3 text-xs text-muted-foreground">
        <div className="flex justify-between gap-2">
          <span>版本</span>
          {freight ? (
            <StageFreightTooltip freight={freight} freightDisplayName={freightDisplayName} />
          ) : (
            <span className="mono truncate text-foreground">{freightDisplayName}</span>
          )}
        </div>
        <div className="flex justify-between gap-2">
          <span>集群</span>
          <span
            className={cn(
              'truncate',
              hasBoundCluster ? 'text-foreground' : 'rounded-full bg-amber-50 px-1.5 py-0.5 text-amber-700'
            )}
            title={hasBoundCluster ? stage.cluster : '未绑定集群，无法发布'}
          >
            {hasBoundCluster ? stage.cluster : '未绑定集群'}
          </span>
        </div>
        <div className="flex items-center justify-between gap-2">
          <span>状态</span>
          <span className="flex min-w-0 shrink-0 items-center gap-1.5">
            <span className={cn('inline-flex items-center gap-1 rounded-full border px-1.5 py-0.5 text-[11px] font-medium', healthMeta.className)} title={`Argo CD 健康状态：${healthMeta.label}（${healthMeta.rawLabel}）`}>
              <HealthIcon className="h-3.5 w-3.5" />
              {healthMeta.label}
            </span>
            <span className={cn('inline-flex items-center gap-1 rounded-full border px-1.5 py-0.5 text-[11px] font-medium', syncMeta.className)} title={`Argo CD 同步状态：${syncMeta.label}（${syncMeta.rawLabel}）`}>
              <SyncIcon className="h-3.5 w-3.5" />
              {syncMeta.label}
            </span>
          </span>
        </div>
      </div>
      {stage.configOutdated && (
        <button
          onClick={(e) => { e.stopPropagation(); onOpenConfigDiff(); }}
          className="mx-3 mt-2 inline-flex cursor-pointer items-center gap-1 rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-amber-700 transition-colors hover:bg-amber-100"
        >
          <CircleDashed className="h-3 w-3" />
          配置已变更
        </button>
      )}
      <div
        data-stage-id={stage.id}
        onDragEnter={(event) => {
          event.preventDefault();
          event.stopPropagation();
          onDragEnter();
        }}
        onDragOver={(event) => {
          event.preventDefault();
          event.stopPropagation();
          event.dataTransfer.dropEffect = canDrop ? 'copy' : 'none';
        }}
        onDragLeave={(event) => {
          event.stopPropagation();
          onDragLeave();
        }}
        onDrop={(event) => {
          event.preventDefault();
          event.stopPropagation();
          onDropFreight(freightIdFromDrag(event));
        }}
        className={cn(
          'nodrag nopan mx-3 mt-4 rounded-md border border-dashed p-3 text-center text-xs transition-colors',
          dragging && canDrop && 'border-primary bg-background text-primary',
          dragging && !canDrop && 'border-slate-300 bg-muted/60 text-muted-foreground',
          !dragging && 'border-[color:var(--stage-chip-border)] bg-background text-muted-foreground'
        )}
      >
        <span className="flex items-center justify-center gap-1.5">
          <CircleDashed className="h-3.5 w-3.5" />
          {dragging && !hasBoundCluster
            ? '未绑定集群，无法发布'
            : dragging && !canDrop
              ? '当前 Freight 不可发布'
              : '拖动 Freight 放置到此处'}
        </span>
      </div>
      <div className="mb-3 mt-3 flex flex-nowrap gap-2 px-3">
        <button
          type="button"
          className="nodrag nopan nowheel inline-flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md border bg-card px-2 text-xs font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
          onPointerDownCapture={(event) => event.stopPropagation()}
          onMouseDownCapture={(event) => event.stopPropagation()}
          onPointerDown={(event) => event.stopPropagation()}
          onMouseDown={(event) => event.stopPropagation()}
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            onOpenConfig();
          }}
        >
          <Settings2 className="h-3.5 w-3.5" />
          配置
        </button>
        <button
          type="button"
          className="nodrag nopan nowheel inline-flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md border bg-card px-2 text-xs font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
          onPointerDownCapture={(event) => event.stopPropagation()}
          onMouseDownCapture={(event) => event.stopPropagation()}
          onPointerDown={(event) => event.stopPropagation()}
          onMouseDown={(event) => event.stopPropagation()}
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            onOpenHistory();
          }}
        >
          <FileText className="h-3.5 w-3.5" />
          历史
        </button>
        <button
          type="button"
          className="nodrag nopan nowheel inline-flex h-8 flex-1 items-center justify-center gap-1.5 rounded-md border bg-card px-2 text-xs font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
          onPointerDownCapture={(event) => event.stopPropagation()}
          onMouseDownCapture={(event) => event.stopPropagation()}
          onPointerDown={(event) => event.stopPropagation()}
          onMouseDown={(event) => event.stopPropagation()}
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            onOpenRuntime();
          }}
        >
          <PanelRightOpen className="h-3.5 w-3.5" />
          详情
        </button>
      </div>
    </div>
  );
}

function stageHasBoundCluster(stage: Stage) {
  return Boolean(stage.clusterBindingId?.trim());
}

function StageHeaderAction({
  label,
  icon,
  badge,
  ariaLabel,
  onClick,
  disabled,
  transparentWhenDisabled
}: {
  label: string;
  icon: ReactNode;
  badge?: number;
  ariaLabel: string;
  onClick: () => void;
  disabled?: boolean;
  transparentWhenDisabled?: boolean;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      className={`nodrag nopan nowheel relative inline-flex h-7 shrink-0 items-center justify-center gap-1 rounded-md border px-2 text-[11px] font-semibold shadow-sm transition-colors ${
        disabled
          ? transparentWhenDisabled
            ? 'cursor-not-allowed border-white/25 bg-transparent text-white/55 shadow-none'
            : 'cursor-not-allowed border-slate-200 bg-slate-100 text-slate-500 shadow-none'
          : 'border-white/35 bg-white/15 text-white hover:bg-white/25'
      }`}
      onPointerDownCapture={(event) => event.stopPropagation()}
      onMouseDownCapture={(event) => event.stopPropagation()}
      onPointerDown={(event) => event.stopPropagation()}
      onMouseDown={(event) => event.stopPropagation()}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onClick();
      }}
      aria-label={ariaLabel}
    >
      {icon}
      {label}
      {Boolean(badge && badge > 0) && (
        <span className="absolute -right-2 -top-2 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-600 px-1 text-[10px] font-bold leading-none text-white shadow-sm ring-1 ring-white/80">
          {badge}
        </span>
      )}
    </button>
  );
}

function ConfigDiffDialog({
  applicationId,
  stage,
  onClose,
  onRedeployed
}: {
  applicationId: string;
  stage: Stage;
  onClose: () => void;
  onRedeployed: () => void;
}) {
  const [loading, setLoading] = useState(true);
  const [diffLines, setDiffLines] = useState<string[]>([]);
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setLoading(true);
    getConfigDiff(applicationId, stage.key)
      .then((result) => setDiffLines(result.diff_lines || []))
      .catch((err) => setError(err instanceof Error ? err.message : '获取配置差异失败'))
      .finally(() => setLoading(false));
  }, [applicationId, stage.key]);

  const handleRedeploy = async () => {
    setSubmitting(true);
    try {
      const token = window.localStorage.getItem('paas_token');
      const actor = token ? { type: 'user', id: 'usr_admin' } : { type: 'user', id: 'usr_admin' };
      await redeployWithConfig(applicationId, stage.key, actor);
      onRedeployed();
    } catch (err) {
      setError(err instanceof Error ? err.message : '重新部署失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className="flex max-h-[80vh] w-[720px] max-w-[calc(100vw-32px)] flex-col overflow-hidden rounded-lg border bg-card shadow-2xl">
        <div className="flex items-center justify-between border-b px-5 py-4">
          <div>
            <h2 className="text-base font-semibold">配置变更 · {stage.name}</h2>
            <p className="mt-0.5 text-xs text-muted-foreground">当前部署的配置与最新配置不一致</p>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose}><X className="h-4 w-4" /></Button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          {loading && <p className="text-sm text-muted-foreground">正在加载配置差异…</p>}
          {error && <p className="text-sm text-red-600">{error}</p>}
          {!loading && !error && diffLines.length === 0 && <p className="text-sm text-muted-foreground">未发现差异</p>}
          {!loading && !error && diffLines.length > 0 && (
            <pre className="overflow-x-auto rounded-md border bg-slate-50 p-3 text-xs leading-5 font-mono">
              {diffLines.map((line, i) => {
                const color = line.startsWith('+') ? 'text-green-700 bg-green-50' : line.startsWith('-') ? 'text-red-700 bg-red-50' : 'text-slate-600';
                return <div key={i} className={color}>{line}</div>;
              })}
            </pre>
          )}
        </div>
        <div className="flex items-center justify-end gap-2 border-t px-5 py-3">
          <Button variant="outline" size="sm" onClick={onClose}>取消</Button>
          <Button size="sm" disabled={submitting || loading || !!error} onClick={handleRedeploy}>
            {submitting ? '提交中…' : '重新部署'}
          </Button>
        </div>
      </div>
    </div>
  );
}

function PromotionConfirmDialog({
  freight,
  stage,
  topologyVersion,
  autoPublish,
  onAutoPublishChange,
  onClose,
  onSubmit
}: {
  freight: Freight;
  stage: Stage;
  topologyVersion: string;
  autoPublish: boolean;
  onAutoPublishChange: (autoPublish: boolean) => void;
  onClose: () => void;
  onSubmit: () => void;
}) {
  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className={DEPLOYMENT_DIALOG_CLASS}>
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <div className="dense-label">创建 Promotion</div>
            <h2 className="mt-1 text-lg font-semibold">确认将 Freight 晋级到 {stage.name}</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              提交前服务端会再次校验拓扑版本、上游 Stage 和审批策略。
            </p>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭 Promotion 确认弹窗">
            <X className="h-4 w-4" />
          </Button>
        </div>

        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-5">
          <div className="rounded-lg border bg-slate-50">
            <div className="flex items-center justify-between gap-3 border-b px-4 py-3">
              <div className="flex min-w-0 items-center gap-3">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border bg-background">
                  <FileArchive className="h-4 w-4 text-primary" />
                </div>
                <div className="min-w-0">
                  <div className="mono truncate text-sm font-semibold">{freightDisplayName(freight)}</div>
                  <div className="text-xs text-muted-foreground">{freight.createdAt}</div>
                </div>
              </div>
            </div>
            <div className="divide-y bg-card">
              {freight.workloads.map((workload) => (
                <FreightWorkloadRow key={workload.name} workload={workload} />
              ))}
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <Metric label="目标 Stage" value={stage.name} />
            <Metric label="拓扑版本" value={topologyVersion} mono />
            <Metric label="晋级策略" value={policyText(stage.promotionPolicy)} />
          </div>

          <label className="flex items-start gap-3 rounded-md border bg-muted/30 p-3 text-sm">
            <input
              type="checkbox"
              checked={autoPublish}
              onChange={(event) => onAutoPublishChange(event.target.checked)}
              className="mt-0.5 h-4 w-4 rounded border-input accent-blue-600"
            />
            <span>
              <span className="block font-medium">自动发布</span>
              <span className="mt-1 block text-xs text-muted-foreground">
                勾选后通过审批或创建完成会自动提交发布；取消勾选后会进入待发布，需要在 Stage 右上角手动发布。
              </span>
            </span>
          </label>
        </div>

        <div className="flex justify-end gap-2 border-t bg-slate-50 px-5 py-4">
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button onClick={onSubmit}>
            <ShieldCheck className="h-4 w-4" />
            创建 Promotion
          </Button>
        </div>
      </div>
    </div>
  );
}

function ApprovalReviewDialog({
  applicationId,
  stage,
  onReviewed,
  onError,
  onClose
}: {
  applicationId: string;
  stage: Stage;
  onReviewed: () => void;
  onError: (message: string) => void;
  onClose: () => void;
}) {
  const [tasks, setTasks] = useState<ApprovalTaskSummary[]>([]);
  const [selectedTaskId, setSelectedTaskId] = useState('');
  const [detail, setDetail] = useState<ApprovalTaskDetail | null>(null);
  const [comment, setComment] = useState('');
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    listApprovalTasks(applicationId, stage.key)
      .then((items) => {
        if (cancelled) return;
        setTasks(items);
        setSelectedTaskId((current) => (items.some((item) => item.id === current) ? current : items[0]?.id || ''));
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : '待审核列表加载失败');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [applicationId, stage.key]);

  useEffect(() => {
    if (!selectedTaskId) {
      setDetail(null);
      return undefined;
    }
    let cancelled = false;
    setDetailLoading(true);
    setError('');
    getApprovalTask(selectedTaskId)
      .then((next) => {
        if (!cancelled) setDetail(next);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : '审核详情加载失败');
      })
      .finally(() => {
        if (!cancelled) setDetailLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedTaskId]);

  async function submit(decision: 'approve' | 'reject') {
    if (!selectedTaskId) return;
    setSubmitting(true);
    try {
      if (decision === 'approve') {
        await approveApprovalTask(selectedTaskId, comment);
      } else {
        await rejectApprovalTask(selectedTaskId, comment);
      }
      setComment('');
      onReviewed();
      const nextTasks = tasks.filter((task) => task.id !== selectedTaskId);
      setTasks(nextTasks);
      setSelectedTaskId(nextTasks[0]?.id || '');
      if (nextTasks.length === 0) onClose();
    } catch (err) {
      const message = err instanceof Error ? err.message : '审核提交失败';
      setError(message);
      onError(message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className={DEPLOYMENT_DIALOG_CLASS}>
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <div className="dense-label">审核发布包</div>
            <h2 className="mt-1 text-lg font-semibold">发布到 {stage.name} 前需要审核</h2>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭审批弹窗">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="grid min-h-0 flex-1 grid-cols-[320px_minmax(0,1fr)] overflow-hidden">
          <aside className="min-h-0 border-r bg-slate-50 p-4">
            <div className="mb-3 text-sm font-semibold text-slate-900">待审核发布包</div>
            {loading && <div className="text-sm text-muted-foreground">加载中...</div>}
            {!loading && tasks.length === 0 && <div className="rounded-md border bg-white p-3 text-sm text-muted-foreground">暂无待审核发布包</div>}
            <div className="space-y-2">
              {tasks.map((task) => (
                <button
                  key={task.id}
                  type="button"
                  className={cn(
                    'block w-full rounded-md border bg-white p-3 text-left text-sm transition-colors hover:border-blue-300 hover:bg-blue-50',
                    selectedTaskId === task.id && 'border-blue-500 bg-blue-50'
                  )}
                  onClick={() => setSelectedTaskId(task.id)}
                >
                  <div className="font-semibold text-slate-900">{task.freightName}</div>
                  <div className="mt-1 text-xs text-muted-foreground">{task.requestedAt}</div>
                  <div className="mt-2 inline-flex rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-semibold text-amber-700">
                    {task.diffType === 'first_deploy' ? '首次部署' : '对比发布'}
                  </div>
                </button>
              ))}
            </div>
          </aside>
          <div className="flex min-h-0 flex-col overflow-hidden p-5">
            {error && <div className="mb-3 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">{error}</div>}
            {detailLoading && <div className="text-sm text-muted-foreground">加载审核详情...</div>}
            {!detailLoading && detail && (
              <div className="flex min-h-0 flex-1 flex-col gap-4">
                <div className="grid grid-cols-2 gap-3">
                  <Metric label="目标 Stage" value={stage.name} />
                  <Metric label="发布类型" value={detail.diffType === 'first_deploy' ? '首次部署' : '对比发布'} />
                </div>
                <DeployFreightVersionCard detail={detail} />
                {detail.diffType === 'first_deploy' ? (
                  <div className="min-h-0 flex-1 overflow-y-auto">
                    <FirstDeployReview detail={detail} />
                  </div>
                ) : (
                  <CompareDeployReview detail={detail} />
                )}
              </div>
            )}
          </div>
        </div>
        <div className="flex items-end gap-3 border-t bg-slate-50 px-5 py-4">
          <div className="flex-1">
            <label className="mb-1 block text-xs font-medium text-slate-600">审核备注</label>
            <textarea
              className="min-h-[72px] w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:border-blue-500"
              value={comment}
              onChange={(event) => setComment(event.target.value)}
              placeholder="请输入通过或驳回原因"
            />
          </div>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button variant="outline" disabled={!selectedTaskId || submitting} onClick={() => void submit('reject')}>
            驳回
          </Button>
          <Button disabled={!selectedTaskId || submitting} onClick={() => void submit('approve')}>
            <ShieldCheck className="h-4 w-4" />
            通过
          </Button>
        </div>
      </div>
    </div>
  );
}

function PublishReviewDialog({
  applicationId,
  stage,
  onPublished,
  onError,
  onClose
}: {
  applicationId: string;
  stage: Stage;
  onPublished: () => void | Promise<void>;
  onError: (message: string) => void;
  onClose: () => void;
}) {
  const [tasks, setTasks] = useState<ApprovalTaskSummary[]>([]);
  const [selectedTaskId, setSelectedTaskId] = useState('');
  const [detail, setDetail] = useState<ApprovalTaskDetail | null>(null);
  const [comment, setComment] = useState('');
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    listPublishTasks(applicationId, stage.key)
      .then((items) => {
        if (cancelled) return;
        setTasks(items);
        setSelectedTaskId((current) => (items.some((item) => item.id === current) ? current : items[0]?.id || ''));
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : '待发布列表加载失败');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [applicationId, stage.key]);

  useEffect(() => {
    if (!selectedTaskId) {
      setDetail(null);
      return undefined;
    }
    let cancelled = false;
    setDetailLoading(true);
    setError('');
    getPublishTask(selectedTaskId)
      .then((next) => {
        if (!cancelled) setDetail(next);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : '发布详情加载失败');
      })
      .finally(() => {
        if (!cancelled) setDetailLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedTaskId]);

  async function submit(decision: 'publish' | 'reject') {
    if (!selectedTaskId) return;
    setSubmitting(true);
    try {
      if (decision === 'publish') {
        await publishTask(selectedTaskId, comment);
      } else {
        await rejectPublishTask(selectedTaskId, comment);
      }
      setComment('');
      await onPublished();
      const nextTasks = tasks.filter((task) => task.id !== selectedTaskId);
      setTasks(nextTasks);
      setSelectedTaskId(nextTasks[0]?.id || '');
      if (nextTasks.length === 0) onClose();
    } catch (err) {
      const message = err instanceof Error ? err.message : '发布提交失败';
      setError(message);
      onError(message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className={DEPLOYMENT_DIALOG_CLASS}>
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <div className="dense-label">手动发布</div>
            <h2 className="mt-1 text-lg font-semibold">发布到 {stage.name} 的待发布任务</h2>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭发布弹窗">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="grid min-h-0 flex-1 grid-cols-[320px_minmax(0,1fr)] overflow-hidden">
          <aside className="min-h-0 border-r bg-slate-50 p-4">
            <div className="mb-3 text-sm font-semibold text-slate-900">待发布发布包</div>
            {loading && <div className="text-sm text-muted-foreground">加载中...</div>}
            {!loading && tasks.length === 0 && <div className="rounded-md border bg-white p-3 text-sm text-muted-foreground">暂无待发布发布包</div>}
            <div className="space-y-2">
              {tasks.map((task) => (
                <button
                  key={task.id}
                  type="button"
                  className={cn(
                    'block w-full rounded-md border bg-white p-3 text-left text-sm transition-colors hover:border-blue-300 hover:bg-blue-50',
                    selectedTaskId === task.id && 'border-blue-500 bg-blue-50'
                  )}
                  onClick={() => setSelectedTaskId(task.id)}
                >
                  <div className="font-semibold text-slate-900">{task.freightName}</div>
                  <div className="mt-1 text-xs text-muted-foreground">{task.requestedAt}</div>
                  <div className="mt-2 inline-flex rounded-full bg-sky-50 px-2 py-0.5 text-[11px] font-semibold text-sky-700">
                    {task.diffType === 'first_deploy' ? '首次部署' : '对比发布'}
                  </div>
                </button>
              ))}
            </div>
          </aside>
          <div className="flex min-h-0 flex-col overflow-hidden p-5">
            {error && <div className="mb-3 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">{error}</div>}
            {detailLoading && <div className="text-sm text-muted-foreground">加载发布详情...</div>}
            {!detailLoading && detail && (
              <div className="flex min-h-0 flex-1 flex-col gap-4">
                <div className="grid grid-cols-2 gap-3">
                  <Metric label="目标 Stage" value={stage.name} />
                  <Metric label="发布类型" value={detail.diffType === 'first_deploy' ? '首次部署' : '对比发布'} />
                </div>
                <DeployFreightVersionCard detail={detail} />
                {detail.diffType === 'first_deploy' ? (
                  <div className="min-h-0 flex-1 overflow-y-auto">
                    <FirstDeployReview detail={detail} />
                  </div>
                ) : (
                  <CompareDeployReview detail={detail} />
                )}
              </div>
            )}
          </div>
        </div>
        <div className="flex items-end gap-3 border-t bg-slate-50 px-5 py-4">
          <div className="flex-1">
            <label className="mb-1 block text-xs font-medium text-slate-600">发布备注</label>
            <textarea
              className="min-h-[72px] w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:border-blue-500"
              value={comment}
              onChange={(event) => setComment(event.target.value)}
              placeholder="请输入发布或驳回原因"
            />
          </div>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button variant="outline" disabled={!selectedTaskId || submitting} onClick={() => void submit('reject')}>
            驳回
          </Button>
          <Button disabled={!selectedTaskId || submitting} onClick={() => void submit('publish')}>
            <Rocket className="h-4 w-4" />
            发布
          </Button>
        </div>
      </div>
    </div>
  );
}

function StageDeploymentHistoryDialog({
  applicationId,
  stage,
  onClose
}: {
  applicationId: string;
  stage: Stage;
  onClose: () => void;
}) {
  const pageSize = 10;
  const [items, setItems] = useState<DeploymentHistoryItem[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [detail, setDetail] = useState<DeploymentHistoryDetail | null>(null);
  const [viewMode, setViewMode] = useState<'diff' | 'yaml'>('diff');
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState('');
  const historyRequestRef = useRef(0);

  const loadHistoryPage = useCallback((nextPage: number, mode: 'replace' | 'append') => {
    const requestId = historyRequestRef.current + 1;
    historyRequestRef.current = requestId;
    const isAppend = mode === 'append';
    if (isAppend) {
      setLoadingMore(true);
    } else {
      setLoading(true);
    }
    setError('');
    return listDeploymentHistory(applicationId, stage.key, nextPage, pageSize)
      .then((result) => {
        if (historyRequestRef.current !== requestId) return;
        setTotal(result.total || 0);
        setPage(result.page || nextPage);
        if (isAppend) {
          setItems((current) => {
            const seen = new Set(current.map((item) => item.deploymentId));
            const additions = result.items.filter((item) => !seen.has(item.deploymentId));
            return [...current, ...additions];
          });
        } else {
          setItems(result.items);
          setSelectedId((current) =>
            result.items.some((item) => item.deploymentId === current) ? current : result.items[0]?.deploymentId || ''
          );
        }
      })
      .catch((err) => {
        if (historyRequestRef.current !== requestId) return;
        setError(err instanceof Error ? err.message : '历史发布记录加载失败');
      })
      .finally(() => {
        if (historyRequestRef.current !== requestId) return;
        if (isAppend) {
          setLoadingMore(false);
        } else {
          setLoading(false);
        }
      });
  }, [applicationId, stage.key]);

  useEffect(() => {
    setItems([]);
    setSelectedId('');
    setDetail(null);
    setPage(1);
    setTotal(0);
    void loadHistoryPage(1, 'replace');
    return () => {
      historyRequestRef.current += 1;
    };
  }, [loadHistoryPage]);

  const handleLoadMoreHistory = () => {
    if (loadingMore || loading || items.length >= total) return;
    void loadHistoryPage(page + 1, 'append');
  };

  useEffect(() => {
    if (!selectedId) {
      setDetail(null);
      return undefined;
    }
    let cancelled = false;
    setDetailLoading(true);
    setError('');
    setViewMode('diff');
    getDeploymentHistoryDetail(selectedId)
      .then((next) => {
        if (!cancelled) setDetail(next);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : '历史详情加载失败');
      })
      .finally(() => {
        if (!cancelled) setDetailLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedId]);

  const code = detail
    ? viewMode === 'yaml'
      ? detail.manifestYaml
      : detail.diffType === 'compare'
        ? detail.configDiff
        : detail.manifestYaml
    : '';
  const emptyDiff = detail && viewMode === 'diff' && detail.diffType === 'compare' && !detail.configDiff;

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className={DEPLOYMENT_DIALOG_CLASS}>
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <div className="dense-label">发布历史</div>
            <h2 className="mt-1 text-lg font-semibold">{stage.name} 的历史发布记录</h2>
            <p className="mt-1 text-xs text-muted-foreground">按 GitOps 提交记录展示；YAML 差异默认对比该 Stage 的上一条成功发布。</p>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭发布历史弹窗">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="grid min-h-0 flex-1 grid-cols-[340px_minmax(0,1fr)] overflow-hidden">
          <aside className="flex min-h-0 flex-col border-r bg-slate-50 p-4">
            <div className="mb-3 flex items-center justify-between gap-2">
              <div className="text-sm font-semibold text-slate-900">历史记录</div>
              <Badge variant="outline">{total > 0 ? `${items.length}/${total} 条` : '0 条'}</Badge>
            </div>
            {loading && <div className="text-sm text-muted-foreground">加载中...</div>}
            {!loading && items.length === 0 && (
              <div className="rounded-md border bg-white p-3 text-sm text-muted-foreground">暂无历史发布记录</div>
            )}
            <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
              {items.map((item) => (
                <button
                  key={item.deploymentId}
                  type="button"
                  className={cn(
                    'block w-full rounded-md border bg-white p-3 text-left text-sm transition-colors hover:border-blue-300 hover:bg-blue-50',
                    selectedId === item.deploymentId && 'border-blue-500 bg-blue-50'
                  )}
                  onClick={() => setSelectedId(item.deploymentId)}
                >
                  <div className="mono truncate font-semibold text-slate-900" title={item.freightName}>{item.freightName}</div>
                  <div className="mt-1 text-xs text-muted-foreground">{item.publishedAt || '发布时间未知'}</div>
                  <div className="mt-2 flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
                    <span className="truncate">发布人：{item.publishedBy || '未知'}</span>
                    <span className="mono shrink-0 rounded-full bg-slate-100 px-2 py-0.5 text-slate-700">{item.commitShort || '无提交'}</span>
                  </div>
                </button>
              ))}
            </div>
            {!loading && items.length < total && (
              <Button
                type="button"
                variant="outline"
                className="mt-3 w-full"
                onClick={handleLoadMoreHistory}
                disabled={loadingMore}
              >
                {loadingMore ? '加载中...' : '加载更多'}
              </Button>
            )}
          </aside>
          <div className="flex min-h-0 flex-col overflow-hidden p-5">
            {error && <div className="mb-3 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">{error}</div>}
            {detailLoading && <div className="text-sm text-muted-foreground">加载历史详情...</div>}
            {!detailLoading && detail && (
              <div className="flex min-h-0 flex-1 flex-col gap-4">
                <div className="grid grid-cols-3 gap-3">
                  <Metric label="发布时间" value={detail.item.publishedAt || '未知'} />
                  <Metric label="发布人" value={detail.item.publishedBy || '未知'} />
                  <Metric label="Git 提交" value={detail.item.commitShort || '无'} mono />
                </div>
                <DeploymentHistoryVersionCard detail={detail} />
                <div className="rounded-md border bg-slate-50 px-4 py-3 text-xs text-muted-foreground">
                  <div className="grid gap-2 md:grid-cols-[120px_minmax(0,1fr)]">
                    <span className="font-medium text-slate-600">Manifest 路径</span>
                    <span className="mono truncate" title={detail.item.manifestPath}>{detail.item.manifestPath || '无'}</span>
                    <span className="font-medium text-slate-600">完整 Commit</span>
                    <span className="mono truncate" title={detail.item.commitSha}>{detail.item.commitSha || '无'}</span>
                  </div>
                </div>
                <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-md border">
                  <div className="flex items-center justify-between gap-3 border-b bg-slate-50 px-4 py-3">
                    <div>
                      <div className="text-sm font-semibold">YAML 记录</div>
                      <div className="mt-0.5 text-xs text-muted-foreground">
                        {detail.diffType === 'compare' ? '当前记录对比上一条成功发布' : '首次发布，暂无上一版可对比'}
                      </div>
                    </div>
                    <div className="inline-flex rounded-md border bg-white p-0.5">
                      <button
                        type="button"
                        className={cn('rounded px-2.5 py-1 text-xs font-medium', viewMode === 'diff' ? 'bg-slate-900 text-white' : 'text-slate-600 hover:bg-slate-100')}
                        onClick={() => setViewMode('diff')}
                      >
                        对比上一版
                      </button>
                      <button
                        type="button"
                        className={cn('rounded px-2.5 py-1 text-xs font-medium', viewMode === 'yaml' ? 'bg-slate-900 text-white' : 'text-slate-600 hover:bg-slate-100')}
                        onClick={() => setViewMode('yaml')}
                      >
                        完整 YAML
                      </button>
                    </div>
                  </div>
                  {emptyDiff ? (
                    <div className="flex flex-1 items-start p-4 text-sm text-muted-foreground">本次发布与上一版 YAML 无变化</div>
                  ) : (
                    <pre className="min-h-[360px] flex-1 overflow-auto bg-slate-950 p-4 text-xs leading-5 text-slate-100">
                      {code.split('\n').map((line, index) => (
                        <div key={`${index}-${line}`} className={cn(line.startsWith('+') && 'text-green-300', line.startsWith('-') && 'text-red-300')}>
                          {line || ' '}
                        </div>
                      ))}
                    </pre>
                  )}
                </div>
              </div>
            )}
            {!loading && !detailLoading && !detail && !error && (
              <div className="rounded-md border bg-slate-50 p-4 text-sm text-muted-foreground">请选择一条历史记录</div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

function FirstDeployReview({ detail }: { detail: ApprovalTaskDetail }) {
  return (
    <div className="space-y-4">
      <div className="rounded-md border border-blue-200 bg-blue-50 p-4 text-sm text-blue-900">
        该 Stage 当前暂无已部署发布包。审核通过后，将首次部署以下发布包到当前 Stage。
      </div>
    </div>
  );
}

function DeploymentHistoryVersionCard({ detail }: { detail: DeploymentHistoryDetail }) {
  const groups = deployItemVersionGroups(detail.deployItems);
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-3 text-left text-xs text-slate-700 shadow-sm">
      <div className="mb-2 border-b border-slate-100 pb-2">
        <div className="text-[11px] font-medium text-slate-500">发布版本</div>
        <div className="mono mt-0.5 truncate font-semibold text-slate-800" title={detail.item.freightName}>
          {detail.item.freightName || detail.item.freightId || '无版本'}
        </div>
      </div>
      {groups.length === 0 ? (
        <div className="rounded-md border border-slate-100 bg-slate-50/70 px-2 py-2 text-slate-500">暂无版本明细</div>
      ) : (
        <div className="max-h-48 space-y-2 overflow-auto pr-1">
          {groups.map((group) => (
            <div key={group.workloadKey} className="rounded-md border border-slate-100 bg-slate-50/70 p-2">
              <div className="mb-1.5 truncate font-medium text-slate-800" title={group.workloadName}>
                {group.workloadName}
              </div>
              <div className="space-y-1.5">
                {group.containers.map((container) => (
                  <div key={`${group.workloadKey}-${container.name}`} className="rounded border border-slate-200 bg-white px-2 py-1.5">
                    <div className="flex items-center justify-between gap-2">
                      <span className="min-w-0 truncate text-slate-600" title={container.name}>{container.name}</span>
                      <span className="mono shrink-0 font-semibold text-slate-800" title={container.version}>{container.version}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function deployItemVersionGroups(items: ApprovalTaskDetail['deployItems']) {
  const groups = new Map<string, {
    workloadKey: string;
    workloadName: string;
    containers: Array<{ name: string; version: string }>;
  }>();
  items.forEach((item) => {
    const key = item.workloadId || item.workloadName || 'workload';
    const container = {
      name: item.containerName || 'app',
      version: compactVersion(versionFromDeployItem(item))
    };
    const existing = groups.get(key);
    if (existing) {
      existing.containers.push(container);
      return;
    }
    groups.set(key, {
      workloadKey: key,
      workloadName: item.workloadName || key,
      containers: [container]
    });
  });
  return [...groups.values()];
}

function DeployFreightVersionCard({ detail }: { detail: ApprovalTaskDetail }) {
  const groups = deployVersionGroups(detail);
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-3 text-left text-xs text-slate-700 shadow-sm">
      <div className="mb-2 border-b border-slate-100 pb-2">
        <div className="text-[11px] font-medium text-slate-500">待发布版本</div>
        <div className="mono mt-0.5 truncate font-semibold text-slate-800" title={detail.pendingFreight.name}>
          {detail.pendingFreight.name || detail.pendingFreight.id || '无版本'}
        </div>
      </div>
      {groups.length === 0 ? (
        <div className="rounded-md border border-slate-100 bg-slate-50/70 px-2 py-2 text-slate-500">暂无版本明细</div>
      ) : (
        <div className="max-h-48 space-y-2 overflow-auto pr-1">
          {groups.map((group) => (
            <div key={group.workloadKey} className="rounded-md border border-slate-100 bg-slate-50/70 p-2">
              <div className="mb-1.5 truncate font-medium text-slate-800" title={group.workloadName}>
                {group.workloadName}
              </div>
              <div className="space-y-1.5">
                {group.containers.map((container) => (
                  <div key={`${group.workloadKey}-${container.name}`} className="rounded border border-slate-200 bg-white px-2 py-1.5">
                    <div className="flex items-center justify-between gap-2">
                      <span className="min-w-0 truncate text-slate-600" title={container.name}>{container.name}</span>
                      <span className="mono shrink-0 font-semibold text-slate-800" title={container.version}>{container.version}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function deployVersionGroups(detail: ApprovalTaskDetail) {
  const groups = new Map<string, {
    workloadKey: string;
    workloadName: string;
    containers: Array<{ name: string; version: string }>;
  }>();

  const add = (workloadKey: string, workloadName: string, containerName: string, version: string) => {
    const key = workloadKey || workloadName || 'workload';
    const existing = groups.get(key);
    const container = {
      name: containerName || 'app',
      version: compactVersion(version)
    };
    if (existing) {
      existing.containers.push(container);
      return;
    }
    groups.set(key, {
      workloadKey: key,
      workloadName: workloadName || key,
      containers: [container]
    });
  };

  if (detail.deployItems.length > 0) {
    detail.deployItems.forEach((item) => {
      add(item.workloadId, item.workloadName, item.containerName, versionFromDeployItem(item));
    });
    return [...groups.values()];
  }

  detail.imageChanges.forEach((change) => {
    add(change.workloadId, change.workloadName, change.containerName, change.pendingVersion);
  });
  return [...groups.values()];
}

function versionFromDeployItem(item: ApprovalTaskDetail['deployItems'][number]) {
  if (item.version && item.version !== '-') return item.version;
  return tagFromImage(item.image);
}

function compactVersion(value: string) {
  const trimmed = String(value || '').trim();
  return trimmed && trimmed !== '-' ? trimmed : '无';
}

function tagFromImage(image: string) {
  const withoutDigest = String(image || '').split('@')[0];
  const tag = withoutDigest.includes(':') ? withoutDigest.split(':').pop() : '';
  return tag || '无';
}

function CompareDeployReview({ detail }: { detail: ApprovalTaskDetail }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      <div className="flex-none rounded-md border">
        <div className="border-b bg-slate-50 px-4 py-3 text-sm font-semibold">镜像版本变化</div>
        {detail.imageChanges.length === 0 ? (
          <div className="p-4 text-sm text-muted-foreground">镜像版本无变化</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50 text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-2">当前版本</th>
                  <th className="px-4 py-2">待发布版本</th>
                </tr>
              </thead>
              <tbody>
                {detail.imageChanges.map((change) => (
                  <tr key={`${change.workloadId}-${change.containerName}`} className="border-t">
                    <td className="px-4 py-2 font-mono text-muted-foreground">{change.currentVersion || '无'}</td>
                    <td className="px-4 py-2 font-mono text-slate-900">{change.pendingVersion || '无'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-md border">
        <div className="border-b bg-slate-50 px-4 py-3 text-sm font-semibold">渲染 YAML 差异</div>
        {detail.configDiff ? (
          <pre className="min-h-[360px] flex-1 overflow-auto bg-slate-950 p-4 text-xs leading-5 text-slate-100">
            {detail.configDiff.split('\n').map((line, index) => (
              <div key={`${index}-${line}`} className={cn(line.startsWith('+') && 'text-green-300', line.startsWith('-') && 'text-red-300')}>
                {line}
              </div>
            ))}
          </pre>
        ) : (
          <div className="flex flex-1 items-start p-4 text-sm text-muted-foreground">渲染 YAML 无变化</div>
        )}
      </div>
    </div>
  );
}

function VerificationDialog({
  stage,
  freight,
  onVerify,
  onClose
}: {
  stage: Stage;
  freight?: Freight | null;
  onVerify: (comment: string) => void;
  onClose: () => void;
}) {
  const [comment, setComment] = useState('');
  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className={DEPLOYMENT_DIALOG_CLASS}>
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <div className="dense-label">人工验证</div>
            <h2 className="mt-1 text-lg font-semibold">验证 {stage.name} 当前发布包</h2>
            <p className="mt-1 text-sm text-muted-foreground">验证结果会影响后续 Stage 是否允许晋级。</p>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭验证弹窗">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-5 space-y-4">
          <div className="grid grid-cols-3 gap-3">
            <Metric label="目标 Stage" value={stage.name} />
            <Metric label="待验证发布包" value={freight ? freightDisplayName(freight) : stage.freightId || '无版本'} />
            <Metric label="目标集群" value={stage.cluster || '未绑定'} />
          </div>
          {stage.checks.length > 0 && (
            <div className="space-y-2">
              <div className="text-sm font-semibold text-slate-900">验证项</div>
              {stage.checks.map((check) => (
                <div key={check} className="flex items-center gap-3 rounded-md border bg-card p-3 text-sm">
                  <CheckCircle2 className="h-4 w-4 text-success" />
                  <span>{check}</span>
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="flex items-end gap-3 border-t bg-slate-50 px-5 py-4">
          <div className="flex-1">
            <label className="mb-1 block text-xs font-medium text-slate-600">验证备注</label>
            <textarea
              className="min-h-[72px] w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:border-blue-500"
              value={comment}
              onChange={(event) => setComment(event.target.value)}
              placeholder="请输入验证备注（可选）"
            />
          </div>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button onClick={() => onVerify(comment)}>
            <ShieldCheck className="h-4 w-4" />
            验证通过
          </Button>
        </div>
      </div>
    </div>
  );
}

function VersionSourceConfigDialog({
  config,
  pipelines,
  onClose,
  onSave
}: {
  config: VersionSourceConfig;
  pipelines: VersionSourcePipeline[];
  onClose: () => void;
  onSave: (config: VersionSourceConfig) => void;
}) {
  const [draft, setDraft] = useState<VersionSourceConfig>(() => cloneVersionSourceConfig(config));
  const [activeWorkloadId, setActiveWorkloadId] = useState(config.workloads[0]?.id || '');
  const activeWorkload = draft.workloads.find((workload) => workload.id === activeWorkloadId) || draft.workloads[0];

  function updateWorkload(workloadId: string, patch: Partial<VersionSourceWorkloadConfig>) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => (
        workload.id === workloadId ? { ...workload, ...patch } : workload
      ))
    }));
  }

  function updateContainerAt(workloadId: string, containerIndex: number, patch: Partial<VersionSourceWorkloadConfig['containers'][number]>) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => (
        workload.id === workloadId
          ? {
              ...workload,
              containers: workload.containers.map((container, index) => (
                index === containerIndex ? { ...container, ...patch } : container
              ))
            }
          : workload
      ))
    }));
  }

  function updateContainerImageSourceAt(workloadId: string, containerIndex: number, imageSource: ContainerImageSource) {
    updateContainerAt(workloadId, containerIndex, { imageSource });
  }

  function updateContainerProbeAt(
    workloadId: string,
    containerIndex: number,
    probeKey: 'livenessProbe' | 'readinessProbe' | 'startupProbe',
    patch: Partial<WorkloadProbeConfig>
  ) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => {
        if (workload.id !== workloadId) return workload;
        return {
          ...workload,
          containers: workload.containers.map((container, index) => {
            if (index !== containerIndex) return container;
            const currentProbe = container[probeKey] || defaultProbeForKey(probeKey, container.port);
            return {
              ...container,
              [probeKey]: { ...currentProbe, ...patch }
            };
          })
        };
      })
    }));
  }

  function addWorkload() {
    const index = draft.workloads.length + 1;
    const workloadName = `新增工作负载 ${index}`;
    const workload: VersionSourceWorkloadConfig = {
      id: `workload-${index}`,
      name: workloadName,
      kind: 'Deployment',
      replicas: 1,
      serviceType: 'ClusterIP',
      servicePort: 8080,
      serverName: workloadName,
      terminationGracePeriodSeconds: 30,
      networkMode: 'container',
      envVars: [],
      secretRefs: [],
      configFiles: [],
      writableDirs: [],
      containers: [{
        id: `workload-${index}-app`,
        name: 'app',
        imageSource: { mode: 'custom', customImage: 'registry.local/app:latest' },
        port: 8080,
        cpu: '250m',
        memory: '256Mi',
        limitCpu: '',
        limitMemory: '',
        command: '',
        envVars: [],
        secretRefs: [],
        configFiles: [],
        writableDirs: [],
        nasMount: { enabled: false, nasPath: '', mountPath: '' },
        ...defaultContainerProbeConfig(8080)
      }]
    };
    setDraft((current) => ({ ...current, workloads: [...current.workloads, workload] }));
    setActiveWorkloadId(workload.id);
  }

  function removeWorkload(workloadId: string) {
    setDraft((current) => {
      const workloads = current.workloads.filter((workload) => workload.id !== workloadId);
      if (activeWorkloadId === workloadId) setActiveWorkloadId(workloads[0]?.id || '');
      return { ...current, workloads };
    });
  }

  function addContainer(workloadId: string) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => {
        if (workload.id !== workloadId) return workload;
        const index = workload.containers.length + 1;
        const existingIds = new Set(workload.containers.map((container) => container.id));
        let nextIndex = index;
        while (existingIds.has(`${workload.id}-container-${nextIndex}`)) nextIndex += 1;
        return {
          ...workload,
          containers: [
            ...workload.containers,
            {
              id: `${workload.id}-container-${nextIndex}`,
              name: `container-${index}`,
              imageSource: { mode: 'custom', customImage: 'registry.local/app:latest' },
              port: 8080,
              cpu: '200m',
              memory: '256Mi',
              limitCpu: '',
              limitMemory: '',
              command: '',
              envVars: [],
              secretRefs: [],
              configFiles: [],
              writableDirs: [],
              nasMount: { enabled: false, nasPath: '', mountPath: '' },
              ...defaultContainerProbeConfig(8080)
            }
          ]
        };
      })
    }));
  }

  function removeContainerAt(workloadId: string, containerIndex: number) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => (
        workload.id === workloadId
          ? { ...workload, containers: workload.containers.filter((_, index) => index !== containerIndex) }
          : workload
      ))
    }));
  }

  function updateContainerListItemAt<K extends 'envVars' | 'secretRefs' | 'configFiles' | 'writableDirs'>(
    workloadId: string,
    containerIndex: number,
    key: K,
    itemIndex: number,
    patch: Record<string, unknown>
  ) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => {
        if (workload.id !== workloadId) return workload;
        return {
          ...workload,
          containers: workload.containers.map((container, index) => {
            if (index !== containerIndex) return container;
            const items = Array.isArray(container[key]) ? [...(container[key] as unknown[])] : [];
            items[itemIndex] = { ...(items[itemIndex] as Record<string, unknown>), ...patch };
            return { ...container, [key]: items };
          })
        };
      })
    }));
  }

  function addContainerListItemAt<K extends 'envVars' | 'secretRefs' | 'configFiles' | 'writableDirs'>(
    workloadId: string,
    containerIndex: number,
    key: K,
    item: Record<string, unknown>
  ) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => {
        if (workload.id !== workloadId) return workload;
        return {
          ...workload,
          containers: workload.containers.map((container, index) => {
            if (index !== containerIndex) return container;
            const items = Array.isArray(container[key]) ? [...(container[key] as unknown[])] : [];
            return { ...container, [key]: [...items, item] };
          })
        };
      })
    }));
  }

  function removeContainerListItemAt<K extends 'envVars' | 'secretRefs' | 'configFiles' | 'writableDirs'>(
    workloadId: string,
    containerIndex: number,
    key: K,
    itemIndex: number
  ) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => {
        if (workload.id !== workloadId) return workload;
        return {
          ...workload,
          containers: workload.containers.map((container, index) => {
            if (index !== containerIndex) return container;
            const items = Array.isArray(container[key]) ? [...(container[key] as unknown[])] : [];
            return { ...container, [key]: items.filter((_, index) => index !== itemIndex) };
          })
        };
      })
    }));
  }

  function updateWorkloadListItem<K extends keyof VersionSourceWorkloadConfig>(
    workloadId: string,
    key: K,
    index: number,
    patch: Record<string, unknown>
  ) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => {
        if (workload.id !== workloadId) return workload;
        const items = Array.isArray(workload[key]) ? [...(workload[key] as unknown[])] : [];
        items[index] = { ...(items[index] as Record<string, unknown>), ...patch };
        return { ...workload, [key]: items };
      })
    }));
  }

  function addWorkloadListItem<K extends keyof VersionSourceWorkloadConfig>(
    workloadId: string,
    key: K,
    item: Record<string, unknown>
  ) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => {
        if (workload.id !== workloadId) return workload;
        const items = Array.isArray(workload[key]) ? [...(workload[key] as unknown[])] : [];
        return { ...workload, [key]: [...items, item] };
      })
    }));
  }

  function removeWorkloadListItem<K extends keyof VersionSourceWorkloadConfig>(
    workloadId: string,
    key: K,
    index: number
  ) {
    setDraft((current) => ({
      ...current,
      workloads: current.workloads.map((workload) => {
        if (workload.id !== workloadId) return workload;
        const items = Array.isArray(workload[key]) ? [...(workload[key] as unknown[])] : [];
        return { ...workload, [key]: items.filter((_, itemIndex) => itemIndex !== index) };
      })
    }));
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className={DEPLOYMENT_DIALOG_CLASS}>
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <div className="dense-label">版本源配置</div>
            <h2 className="mt-1 text-lg font-semibold">部署配置</h2>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭版本源配置">
            <X className="h-4 w-4" />
          </Button>
        </div>

        <div className="grid min-h-0 flex-1 grid-cols-[248px_minmax(0,1fr)] overflow-hidden">
          <aside className="border-r bg-slate-50 p-4">
            <div className="mb-3 flex items-center justify-between">
              <div className="text-sm font-semibold">工作负载</div>
              <Button size="sm" variant="outline" onClick={addWorkload}>
                <Plus className="h-3.5 w-3.5" />
                添加
              </Button>
            </div>
            <div className="space-y-2">
              {draft.workloads.map((workload) => (
                <button
                  key={workload.id}
                  type="button"
                  onClick={() => setActiveWorkloadId(workload.id)}
                  className={cn(
                    'w-full rounded-md border bg-card p-3 text-left transition-colors hover:border-primary/40',
                    activeWorkload?.id === workload.id && 'border-primary bg-accent'
                  )}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0 space-y-1">
                      <div className="truncate text-sm font-semibold">{workload.name}</div>
                      <div className="flex flex-wrap gap-1">
                        <Badge variant="outline">{workloadKindLabel(workload.kind)}</Badge>
                        <Badge variant="secondary">{workload.containers.length} 镜像</Badge>
                      </div>
                    </div>
                    <Layers3 className="h-4 w-4 text-muted-foreground" />
                  </div>
                </button>
              ))}
            </div>
          </aside>

          <div className="min-h-0 overflow-y-auto bg-white">
            {activeWorkload ? (
              <div className="divide-y">
                <ConfigSection
                  icon={<Server className="h-4 w-4" />}
                  title="控制器与访问入口"
                  description="先定义工作负载的运行方式，再决定是否暴露服务入口。"
                  action={(
                    <Button variant="outline" size="sm" onClick={() => removeWorkload(activeWorkload.id)} disabled={draft.workloads.length <= 1}>
                      <Trash2 className="h-3.5 w-3.5" />
                      删除工作负载
                    </Button>
                  )}
                >
                  <div className="space-y-4">
                    <div className="rounded-md border bg-white p-4">
                      <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
                        <Layers3 className="h-4 w-4 text-primary" />
                        基础信息
                      </div>
                      <div className="grid gap-3">
                        <CompactField label="名称">
                          <Input value={activeWorkload.name} onChange={(event) => updateWorkload(activeWorkload.id, { name: event.target.value })} />
                        </CompactField>
                        <CompactField label="类型">
                          <select
                            value={activeWorkload.kind}
                            onChange={(event) => updateWorkload(activeWorkload.id, { kind: event.target.value as VersionSourceWorkloadConfig['kind'] })}
                            className="h-9 w-full rounded-md border bg-card px-3 text-sm"
                          >
                            <option value="Deployment">无状态</option>
                            <option value="StatefulSet">有状态</option>
                          </select>
                        </CompactField>
                        <CompactField label="副本数">
                          <NumberTextInput value={activeWorkload.replicas} min={0} defaultValue={1} onValueChange={(value) => updateWorkload(activeWorkload.id, { replicas: value })} />
                        </CompactField>
                      </div>
                    </div>

                    <div className="rounded-md border bg-white p-4">
                      <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
                        <Globe2 className="h-4 w-4 text-primary" />
                        访问入口
                      </div>
                      {(() => {
                        const accessDisabled = activeWorkload.serviceType === 'None';
                        const disabledInputClass = accessDisabled ? 'opacity-50' : '';
                        return (
                          <div className="space-y-3">
                            <div className="grid gap-3">
                              <CompactField label="服务类型">
                                <select
                                  value={activeWorkload.serviceType || 'ClusterIP'}
                                  onChange={(event) => updateWorkload(activeWorkload.id, { serviceType: event.target.value as VersionSourceWorkloadConfig['serviceType'] })}
                                  className="h-9 w-full rounded-md border bg-card px-3 text-sm"
                                >
                                  <option value="None">不提供对外访问</option>
                                  <option value="ClusterIP">集群内访问</option>
                                  <option value="NodePort">节点端口</option>
                                  <option value="LoadBalancer">负载均衡</option>
                                </select>
                              </CompactField>
                              <CompactField label="服务名">
                                <Input
                                  value={activeWorkload.serverName || activeWorkload.name}
                                  disabled={accessDisabled}
                                  className={disabledInputClass}
                                  onChange={(event) => updateWorkload(activeWorkload.id, { serverName: event.target.value })}
                                  placeholder={activeWorkload.name}
                                />
                              </CompactField>
                              <CompactField label="服务端口">
                                <NumberTextInput
                                  value={activeWorkload.servicePort}
                                  min={1}
                                  max={65535}
                                  defaultValue={8080}
                                  className={disabledInputClass}
                                  disabled={accessDisabled}
                                  onValueChange={(value) => updateWorkload(activeWorkload.id, { servicePort: value })}
                                />
                              </CompactField>
                            </div>

                            <label className={`flex h-9 items-center gap-2 rounded-md border bg-card px-3 text-sm ${accessDisabled ? 'text-muted-foreground opacity-60' : ''}`}>
                              <input
                                type="checkbox"
                                checked={!!activeWorkload.enableDomainAccess}
                                disabled={accessDisabled}
                                onChange={(event) => updateWorkload(activeWorkload.id, { enableDomainAccess: event.target.checked })}
                              />
                              域名访问
                            </label>

                            {activeWorkload.enableDomainAccess ? (
                              <div className={`grid gap-3 rounded-md border bg-white p-3 ${accessDisabled ? 'opacity-50' : ''}`}>
                                <CompactField label="域名">
                                  <Input
                                    value={activeWorkload.domain || ''}
                                    disabled={accessDisabled}
                                    onChange={(event) => updateWorkload(activeWorkload.id, { domain: event.target.value })}
                                    placeholder="dev-order.example.com"
                                  />
                                </CompactField>
                                <CompactField label="路径">
                                  <Input
                                    value={activeWorkload.ingressPath || '/'}
                                    disabled={accessDisabled}
                                    onChange={(event) => updateWorkload(activeWorkload.id, { ingressPath: event.target.value })}
                                  />
                                </CompactField>
                                <label className="flex h-9 items-center gap-2 rounded-md border bg-card px-3 text-sm">
                                  <input
                                    type="checkbox"
                                    checked={!!activeWorkload.ingressRewrite}
                                    disabled={accessDisabled}
                                    onChange={(event) => updateWorkload(activeWorkload.id, { ingressRewrite: event.target.checked })}
                                  />
                                  是否重写
                                </label>
                                {activeWorkload.ingressRewrite ? (
                                  <CompactField label="重写路径">
                                    <Input
                                      value={activeWorkload.ingressRewritePath || '/'}
                                      disabled={accessDisabled}
                                      onChange={(event) => updateWorkload(activeWorkload.id, { ingressRewritePath: event.target.value })}
                                    />
                                  </CompactField>
                                ) : null}
                                <label className="flex h-9 items-center gap-2 rounded-md border bg-card px-3 text-sm">
                                  <input
                                    type="checkbox"
                                    checked={!!activeWorkload.ingressTls}
                                    disabled={accessDisabled}
                                    onChange={(event) => updateWorkload(activeWorkload.id, { ingressTls: event.target.checked })}
                                  />
                                  HTTPS
                                </label>
                                {activeWorkload.ingressTls ? (
                                  <label className="flex h-9 items-center gap-2 rounded-md border bg-card px-3 text-sm">
                                    <input
                                      type="checkbox"
                                      checked={!!activeWorkload.ingressTlsRedirect}
                                      disabled={accessDisabled}
                                      onChange={(event) => updateWorkload(activeWorkload.id, { ingressTlsRedirect: event.target.checked })}
                                    />
                                    HTTP 重定向到 HTTPS
                                  </label>
                                ) : null}
                              </div>
                            ) : null}
                          </div>
                        );
                      })()}
                    </div>
                  </div>
                </ConfigSection>

                <ConfigSection
                  icon={<Box className="h-4 w-4" />}
                  title="容器配置"
                  description="容器是镜像、资源、探针和运行配置的最小单位。"
                  action={(
                    <Button size="sm" variant="outline" onClick={() => addContainer(activeWorkload.id)}>
                      <Plus className="h-3.5 w-3.5" />
                      添加容器
                    </Button>
                  )}
                >
                  <div className="space-y-3">
                    {activeWorkload.containers.map((container, containerIndex) => (
                      <div key={`${container.id}-${containerIndex}`} className="overflow-hidden rounded-xl border bg-white shadow-sm">
                        <div className="flex items-center justify-between gap-3 border-b bg-white px-4 py-3">
                          <div className="flex min-w-0 items-center gap-2">
                            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border bg-slate-50 text-primary">
                              <Box className="h-4 w-4" />
                            </div>
                            <div className="min-w-0">
                              <div className="truncate text-sm font-semibold">{container.name}</div>
                              <div className="mt-0.5 text-xs text-muted-foreground">
                                {container.imageSource.mode === 'pipeline' ? '流水线镜像' : '自定义镜像'} · {container.port} 端口
                              </div>
                            </div>
                          </div>
                          <Button variant="ghost" size="sm" onClick={() => removeContainerAt(activeWorkload.id, containerIndex)} disabled={activeWorkload.containers.length <= 1}>
                            <Trash2 className="h-3.5 w-3.5" />
                            删除
                          </Button>
                        </div>

                        <div className="space-y-4 p-4">
                          <div className="rounded-md border bg-white p-4">
                            <div className="mb-3 text-xs font-semibold text-muted-foreground">镜像与资源</div>
                              <div className="grid gap-3">
                              <CompactField label="容器名称">
                                <Input value={container.name} onChange={(event) => updateContainerAt(activeWorkload.id, containerIndex, { name: event.target.value })} />
                              </CompactField>
                              <CompactField label="启动命令">
                                <div className="space-y-1.5">
                                  <Input
                                    value={container.command || ''}
                                    onChange={(event) => updateContainerAt(activeWorkload.id, containerIndex, { command: event.target.value })}
                                    placeholder="例如：java -jar app.jar"
                                  />
                                  <div className="text-[11px] text-muted-foreground">不填则用镜像默认启动命令</div>
                                </div>
                              </CompactField>
                              <CompactField label="镜像来源">
                                <select
                                  value={container.imageSource.mode}
                                  onChange={(event) => {
                                    const mode = event.target.value as ContainerImageSource['mode'];
                                    updateContainerImageSourceAt(activeWorkload.id, containerIndex, mode === 'pipeline'
                                      ? { mode, pipelineId: pipelines[0]?.id }
                                      : { mode, customImage: container.imageSource.customImage || 'registry.local/app:latest' });
                                  }}
                                  className="h-9 w-full rounded-md border bg-card px-3 text-sm"
                                >
                                  <option value="pipeline">关联流水线</option>
                                  <option value="custom">自定义镜像</option>
                                </select>
                              </CompactField>
                              {container.imageSource.mode === 'pipeline' ? (
                                <CompactField label="关联流水线">
                                  <select
                                    value={container.imageSource.pipelineId || ''}
                                    onChange={(event) => updateContainerImageSourceAt(activeWorkload.id, containerIndex, { mode: 'pipeline', pipelineId: event.target.value })}
                                    className="h-9 w-full rounded-md border bg-card px-3 text-sm"
                                  >
                                    {pipelines.map((pipeline) => (
                                      <option key={pipeline.id} value={pipeline.id}>{pipeline.name}</option>
                                    ))}
                                  </select>
                                </CompactField>
                              ) : (
                                <CompactField label="自定义镜像">
                                  <Input
                                    value={container.imageSource.customImage || ''}
                                    onChange={(event) => updateContainerImageSourceAt(activeWorkload.id, containerIndex, { mode: 'custom', customImage: event.target.value })}
                                    placeholder="registry.local/app:tag"
                                  />
                                </CompactField>
                              )}
                              <div className="grid gap-3 sm:grid-cols-2">
                                <CompactField label="容器端口">
                                  <NumberTextInput value={container.port} min={1} defaultValue={8080} onValueChange={(value) => updateContainerAt(activeWorkload.id, containerIndex, { port: value })} />
                                </CompactField>
                                <CompactField label="CPU 请求">
                                  <Input value={container.cpu} onChange={(event) => updateContainerAt(activeWorkload.id, containerIndex, { cpu: event.target.value })} />
                                </CompactField>
                                <CompactField label="内存请求">
                                  <Input value={container.memory} onChange={(event) => updateContainerAt(activeWorkload.id, containerIndex, { memory: event.target.value })} />
                                </CompactField>
                                <CompactField label="CPU 限制">
                                  <Input value={container.limitCpu || ''} onChange={(event) => updateContainerAt(activeWorkload.id, containerIndex, { limitCpu: event.target.value })} placeholder="2" />
                                </CompactField>
                                <CompactField label="内存限制">
                                  <Input value={container.limitMemory || ''} onChange={(event) => updateContainerAt(activeWorkload.id, containerIndex, { limitMemory: event.target.value })} placeholder="2Gi" />
                                </CompactField>
                              </div>
                            </div>
                          </div>

                          <div className="rounded-md border bg-white p-4">
                          <div className="mb-3 flex items-start justify-between gap-3">
                            <div>
                              <div className="text-xs font-semibold text-muted-foreground">容器探针</div>
                              <div className="mt-1 text-[11px] text-muted-foreground">存活、就绪和启动探针都作用于当前容器。</div>
                            </div>
                            <label className="flex h-8 items-center gap-2 rounded-md border bg-slate-50 px-3 text-xs">
                              <input
                                type="checkbox"
                                checked={!!(container.startupProbe?.enabled)}
                                onChange={(event) => updateContainerProbeAt(activeWorkload.id, containerIndex, 'startupProbe', {
                                  ...defaultProbeForKey('startupProbe', container.port),
                                  ...container.startupProbe,
                                  enabled: event.target.checked
                                })}
                              />
                              启用启动探针
                            </label>
                          </div>
                          <div className="grid gap-3 2xl:grid-cols-2">
                            <ContainerProbeFields
                              title="存活探针"
                              probe={container.livenessProbe || defaultProbeForKey('livenessProbe', container.port)}
                              onChange={(patch) => updateContainerProbeAt(activeWorkload.id, containerIndex, 'livenessProbe', patch)}
                            />
                            <ContainerProbeFields
                              title="就绪探针"
                              probe={container.readinessProbe || defaultProbeForKey('readinessProbe', container.port)}
                              onChange={(patch) => updateContainerProbeAt(activeWorkload.id, containerIndex, 'readinessProbe', patch)}
                            />
                            {container.startupProbe?.enabled && (
                              <div className="xl:col-span-2">
                                <ContainerProbeFields
                                  title="启动探针"
                                  probe={container.startupProbe || defaultProbeForKey('startupProbe', container.port)}
                                  onChange={(patch) => updateContainerProbeAt(activeWorkload.id, containerIndex, 'startupProbe', patch)}
                                />
                              </div>
                            )}
                          </div>
                          </div>

                        <div className="rounded-lg border bg-slate-50/70 p-3 xl:col-span-2">
                          <div className="mb-3">
                            <div className="text-xs font-semibold text-muted-foreground">容器配置</div>
                            <div className="mt-1 text-[11px] text-muted-foreground">环境变量、敏感配置、配置文件和可写目录都作用于当前容器。</div>
                          </div>
                          <div className="grid gap-4">
                            <ConfigList
                              title="环境变量"
                              addLabel="添加"
                              items={container.envVars || []}
                              emptyText="暂无环境变量"
                              onAdd={() => addContainerListItemAt(activeWorkload.id, containerIndex, 'envVars', { id: `env-${Date.now()}`, name: '', value: '' })}
                              onRemove={(index) => removeContainerListItemAt(activeWorkload.id, containerIndex, 'envVars', index)}
                              render={(item, index) => (
                                <>
                                  <Input value={String(item.name || '')} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'envVars', index, { name: event.target.value })} placeholder="SPRING_PROFILES_ACTIVE" />
                                  <Input value={String(item.value || '')} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'envVars', index, { value: event.target.value })} placeholder="prod" />
                                </>
                              )}
                            />
                            <ConfigList
                              title="敏感配置"
                              addLabel="添加"
                              items={container.secretRefs || []}
                              emptyText="暂无敏感配置"
                              onAdd={() => addContainerListItemAt(activeWorkload.id, containerIndex, 'secretRefs', { id: `secret-${Date.now()}`, name: '', secretRef: '' })}
                              onRemove={(index) => removeContainerListItemAt(activeWorkload.id, containerIndex, 'secretRefs', index)}
                              render={(item, index) => (
                                <>
                                  <Input value={String(item.name || '')} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'secretRefs', index, { name: event.target.value })} placeholder="DB_PASSWORD" />
                                  <Input value={String((item as { secretRef?: string }).secretRef || '')} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'secretRefs', index, { secretRef: event.target.value })} placeholder="secret/data/app/db" />
                                </>
                              )}
                            />
                            <ConfigList
                              title="配置文件"
                              addLabel="添加"
                              items={container.configFiles || []}
                              emptyText="暂无配置文件"
                              onAdd={() => addContainerListItemAt(activeWorkload.id, containerIndex, 'configFiles', { id: `config-${Date.now()}`, mountPath: '', content: '', base64Encoded: false })}
                              onRemove={(index) => removeContainerListItemAt(activeWorkload.id, containerIndex, 'configFiles', index)}
                              render={(item, index) => (
                                <>
                                  <Input value={String((item as { mountPath?: string }).mountPath || '')} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'configFiles', index, { mountPath: event.target.value })} placeholder="/etc/app/app.yaml" />
                                  <textarea value={String((item as { content?: string }).content || '')} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'configFiles', index, { content: event.target.value })} className="min-h-20 rounded-md border bg-card px-3 py-2 text-sm" placeholder="server.port: 8080" />
                                  <label className="flex items-center gap-2 text-xs text-muted-foreground">
                                    <input type="checkbox" checked={!!(item as { base64Encoded?: boolean }).base64Encoded} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'configFiles', index, { base64Encoded: event.target.checked })} />
                                    Base64 编码
                                  </label>
                                </>
                              )}
                            />
                            <ConfigList
                              title="可写目录"
                              addLabel="添加"
                              items={container.writableDirs || []}
                              emptyText="暂无可写目录"
                              onAdd={() => addContainerListItemAt(activeWorkload.id, containerIndex, 'writableDirs', { id: `dir-${Date.now()}`, mountPath: '', ownerGroup: '', mode: '', sizeLimit: '' })}
                              onRemove={(index) => removeContainerListItemAt(activeWorkload.id, containerIndex, 'writableDirs', index)}
                              render={(item, index) => (
                                <>
                                  <Input value={String((item as { mountPath?: string }).mountPath || '')} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'writableDirs', index, { mountPath: event.target.value })} placeholder="/data" />
                                  <Input value={String((item as { ownerGroup?: string }).ownerGroup || '')} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'writableDirs', index, { ownerGroup: event.target.value })} placeholder="app:app" />
                                  <Input value={String((item as { mode?: string }).mode || '')} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'writableDirs', index, { mode: event.target.value })} placeholder="0775" />
                                  <Input value={String((item as { sizeLimit?: string }).sizeLimit || '')} onChange={(event) => updateContainerListItemAt(activeWorkload.id, containerIndex, 'writableDirs', index, { sizeLimit: event.target.value })} placeholder="5Gi" />
                                </>
                              )}
                            />
                            <div className="rounded-md border bg-white p-3">
                              <label className="flex items-center gap-2 text-sm font-medium text-foreground">
                                <input
                                  type="checkbox"
                                  checked={!!container.nasMount?.enabled}
                                  onChange={(event) => updateContainerAt(activeWorkload.id, containerIndex, {
                                    nasMount: {
                                      enabled: event.target.checked,
                                      nasPath: container.nasMount?.nasPath || '',
                                      mountPath: container.nasMount?.mountPath || ''
                                    }
                                  })}
                                />
                                挂载 NAS
                              </label>
                              {container.nasMount?.enabled && (
                                <div className="mt-3 grid gap-2">
                                  <Input
                                    value={container.nasMount?.nasPath || ''}
                                    onChange={(event) => updateContainerAt(activeWorkload.id, containerIndex, {
                                      nasMount: {
                                        enabled: true,
                                        nasPath: event.target.value,
                                        mountPath: container.nasMount?.mountPath || ''
                                      }
                                    })}
                                    placeholder="NAS 路径，例如 /share/app-data"
                                  />
                                  <Input
                                    value={container.nasMount?.mountPath || ''}
                                    onChange={(event) => updateContainerAt(activeWorkload.id, containerIndex, {
                                      nasMount: {
                                        enabled: true,
                                        nasPath: container.nasMount?.nasPath || '',
                                        mountPath: event.target.value
                                      }
                                    })}
                                    placeholder="容器内路径，例如 /mnt/nas"
                                  />
                                </div>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                    ))}
                  </div>
                </ConfigSection>

                <ConfigSection
                  icon={<Settings2 className="h-4 w-4" />}
                  title="高级配置"
                  description="用于设置工作负载调度策略和终止等待时间。"
                >
                  <div className="grid gap-3">
                    <CompactField label={<span className="inline-flex items-center gap-1">节点类型 <span className="relative group"><HelpCircle className="h-3.5 w-3.5 text-muted-foreground cursor-help" /><span className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1 -translate-x-1/2 whitespace-nowrap rounded bg-foreground px-2 py-1 text-xs text-background opacity-0 group-hover:opacity-100 transition-opacity">决定调度到不同类型的服务器上</span></span></span>}>
                      <div className="flex gap-4">
                        {([['general', '通用'], ['network', '网络'], ['memory', '内存'], ['compute', '计算']] as const).map(([value, label]) => (
                          <label key={value} className="flex items-center gap-1.5 text-sm">
                            <input type="radio" name={`nodeType-${activeWorkload.id}`} value={value} checked={(activeWorkload.nodeType || 'general') === value} onChange={() => updateWorkload(activeWorkload.id, { nodeType: value })} />
                            {label}
                          </label>
                        ))}
                      </div>
                    </CompactField>
                    <CompactField label={<span className="inline-flex items-center gap-1">是否独占 <span className="relative group"><HelpCircle className="h-3.5 w-3.5 text-muted-foreground cursor-help" /><span className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1 -translate-x-1/2 whitespace-nowrap rounded bg-foreground px-2 py-1 text-xs text-background opacity-0 group-hover:opacity-100 transition-opacity">独占表示节点只有该工作负载可以调度，如果独占节点未准备好，会出现实例 Pending</span></span></span>}>
                      <label className="flex items-center gap-2 text-sm">
                        <input type="checkbox" checked={!!activeWorkload.exclusive} onChange={(e) => updateWorkload(activeWorkload.id, { exclusive: e.target.checked })} />
                        独占节点
                      </label>
                    </CompactField>
                    <CompactField label="终止等待">
                      <NumberTextInput value={activeWorkload.terminationGracePeriodSeconds ?? 30} min={0} defaultValue={30} onValueChange={(value) => updateWorkload(activeWorkload.id, { terminationGracePeriodSeconds: value })} />
                    </CompactField>
                    <CompactField label="网络模式">
                      <select
                        value={activeWorkload.networkMode || 'container'}
                        onChange={(event) => updateWorkload(activeWorkload.id, { networkMode: event.target.value as VersionSourceWorkloadConfig['networkMode'] })}
                        className="h-9 rounded-md border border-border bg-background px-3 text-sm"
                      >
                        <option value="container">容器网络</option>
                        <option value="host">Host 网络</option>
                      </select>
                    </CompactField>
                  </div>
                </ConfigSection>
              </div>
            ) : (
              <div className="flex min-h-[320px] items-center justify-center text-sm text-muted-foreground">暂无工作负载配置</div>
            )}
          </div>

        </div>

        <div className="flex justify-end gap-2 border-t bg-slate-50 px-5 py-4">
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button onClick={() => onSave(draft)}>
            <CheckCircle2 className="h-4 w-4" />
            保存配置
          </Button>
        </div>
      </div>
    </div>
  );
}

const fallbackRuntimeEnvironmentOptions: PipelineEnvironmentOption[] = ['Java 17 / Maven', 'Java 17 / Gradle', 'Node 22 / pnpm', 'Go 1.23', 'Python 3.12']
  .map((name) => ({ id: name, name }));
const fallbackBuildEnvironmentOptions: PipelineEnvironmentOption[] = ['Maven JDK17 构建环境', 'Gradle JDK17 构建环境', 'Node 22 构建环境', 'Go 构建环境', 'Python 构建环境']
  .map((name) => ({ id: name, name }));
type PipelineSourceConfig = VersionSourcePipeline['sources'][number];

function nextPipelineSource(index: number, buildOptions: PipelineEnvironmentOption[] = fallbackBuildEnvironmentOptions): PipelineSourceConfig {
  const buildEnvironment = buildOptions[0] || fallbackBuildEnvironmentOptions[0];
  return {
    id: `src-${Date.now().toString(36)}-${index}`,
    key: index === 0 ? 'main' : `source-${index + 1}`,
    name: index === 0 ? '主代码源' : `代码源 ${index + 1}`,
    sourceType: 'git',
    repository: 'https://gitlab.internal/retail/order-platform.git',
    sourceUrl: 'https://gitlab.internal/retail/order-platform.git',
    sourceRef: 'main',
    svnCheckoutPaths: [{ local: '.', path: '', depth: 'infinity' }],
    branch: 'main',
    sourcePath: '.',
    buildEnvironment: buildEnvironment.name,
    buildEnvironmentId: buildEnvironment.id,
    buildCommand: 'mvn clean package -DskipTests',
    artifactCopyCommand: 'cp -ar target/*.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"'
  };
}

function pipelineFromForm(pipeline: VersionSourcePipeline | null, input: {
  name: string;
  runtime: string;
  runtimeEnvironmentId: string;
  sources: PipelineSourceConfig[];
}): VersionSourcePipeline {
  const primarySource = input.sources[0] || nextPipelineSource(0);
  return {
    id: pipeline?.id || `pipe-custom-${Date.now().toString(36)}`,
    name: input.name,
    branch: primarySource.sourceRef || primarySource.branch,
    runtime: input.runtime,
    runtimeEnvironmentIds: input.runtimeEnvironmentId ? [input.runtimeEnvironmentId] : [],
    sourcePath: primarySource.sourcePath,
    buildCommand: primarySource.buildCommand,
    artifactCopyCommand: primarySource.artifactCopyCommand,
    sources: input.sources,
    buildHistory: pipeline?.buildHistory || [],
    logs: pipeline?.logs || [],
    latestVersion: pipeline?.latestVersion || '暂无版本',
    status: pipeline?.status || 'pending'
  };
}

function optionById(options: PipelineEnvironmentOption[], id: string) {
  return options.find((option) => option.id === id);
}

function optionByName(options: PipelineEnvironmentOption[], name: string) {
  const needle = name.trim().toLowerCase();
  return options.find((option) => option.name.trim().toLowerCase() === needle || option.id.trim().toLowerCase() === needle);
}

function PipelineCreateDialog({
  pipelines,
  runtimeOptions,
  buildEnvironmentOptions,
  onClose,
  onCreate
}: {
  pipelines: VersionSourcePipeline[];
  runtimeOptions: PipelineEnvironmentOption[];
  buildEnvironmentOptions: PipelineEnvironmentOption[];
  onClose: () => void;
  onCreate: (pipeline: VersionSourcePipeline) => void;
}) {
  const index = pipelines.length + 1;
  const [name, setName] = useState(`新增流水线 ${index}`);
  const [runtimeId, setRuntimeId] = useState(runtimeOptions[0]?.id || '');
  const [sources, setSources] = useState<PipelineSourceConfig[]>([nextPipelineSource(0, buildEnvironmentOptions)]);
  const selectedRuntime = optionById(runtimeOptions, runtimeId) || runtimeOptions[0] || fallbackRuntimeEnvironmentOptions[0];

  function submit() {
    onCreate(pipelineFromForm(null, {
      name,
      runtime: selectedRuntime.name,
      runtimeEnvironmentId: selectedRuntime.id,
      sources
    }));
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className={DEPLOYMENT_DIALOG_CLASS}>
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <div className="dense-label">新增流水线</div>
            <h2 className="mt-1 text-lg font-semibold">添加构建流水线到画布</h2>
            <p className="mt-1 text-sm text-muted-foreground">参考旧版：运行时环境单选，代码源可添加多条，字段按单列填写。</p>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭新增流水线弹窗">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto p-5">
          <InlineField label="显示名称">
            <Input value={name} onChange={(event) => setName(event.target.value)} />
          </InlineField>
          <InlineField label="运行时环境">
            <select value={runtimeId} onChange={(event) => setRuntimeId(event.target.value)} className="h-10 w-full rounded-md border bg-card px-3 text-sm">
              {runtimeOptions.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
            </select>
          </InlineField>
          <PipelineSourcesEditor sources={sources} buildEnvironmentOptions={buildEnvironmentOptions} onChange={setSources} />
        </div>
        <div className="flex justify-end gap-2 border-t bg-slate-50 px-5 py-4">
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button onClick={submit} disabled={!name.trim()}>
            <Plus className="h-4 w-4" />
            添加流水线
          </Button>
        </div>
      </div>
    </div>
  );
}

function PipelineConfigDialog({
  pipeline,
  runtimeOptions,
  buildEnvironmentOptions,
  onClose,
  onSave
}: {
  pipeline: VersionSourcePipeline;
  runtimeOptions: PipelineEnvironmentOption[];
  buildEnvironmentOptions: PipelineEnvironmentOption[];
  onClose: () => void;
  onSave: (pipeline: VersionSourcePipeline) => void;
}) {
  const [name, setName] = useState(pipeline.name);
  const [runtimeId, setRuntimeId] = useState(pipeline.runtimeEnvironmentIds?.[0] || optionByName(runtimeOptions, pipeline.runtime)?.id || runtimeOptions[0]?.id || '');
  const [sources, setSources] = useState<PipelineSourceConfig[]>(
    pipeline.sources?.length ? pipeline.sources : [nextPipelineSource(0, buildEnvironmentOptions)]
  );
  const selectedRuntime = optionById(runtimeOptions, runtimeId) || optionByName(runtimeOptions, pipeline.runtime) || runtimeOptions[0] || { id: pipeline.runtimeEnvironmentIds?.[0] || pipeline.runtime, name: pipeline.runtime };

  function submit() {
    onSave(pipelineFromForm(pipeline, {
      name,
      runtime: selectedRuntime.name,
      runtimeEnvironmentId: selectedRuntime.id,
      sources
    }));
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className={DEPLOYMENT_DIALOG_CLASS}>
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <div className="dense-label">流水线配置</div>
            <h2 className="mt-1 text-lg font-semibold">{pipeline.name}</h2>
            <p className="mt-1 text-sm text-muted-foreground">运行时环境为单选；每条代码源维护自己的分支、目录和构建命令。</p>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭流水线配置弹窗">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto p-5">
          <InlineField label="显示名称">
            <Input value={name} onChange={(event) => setName(event.target.value)} />
          </InlineField>
          <InlineField label="运行时环境">
            <select value={runtimeId} onChange={(event) => setRuntimeId(event.target.value)} className="h-10 w-full rounded-md border bg-card px-3 text-sm">
              {runtimeOptions.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
            </select>
          </InlineField>
          <PipelineSourcesEditor sources={sources} buildEnvironmentOptions={buildEnvironmentOptions} onChange={setSources} />
        </div>
        <div className="flex justify-end gap-2 border-t bg-slate-50 px-5 py-4">
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button onClick={submit} disabled={!name.trim()}>
            <Settings2 className="h-4 w-4" />
            保存配置
          </Button>
        </div>
      </div>
    </div>
  );
}

function PipelineBuildDialog({
  pipeline,
  onClose,
  onSubmit,
  onCancelBuild,
  onStreamLog,
  onBuildStatusChange
}: {
  pipeline: VersionSourcePipeline;
  onClose: () => void;
  onSubmit: (pipeline: VersionSourcePipeline, sourceRef: string, buildCommand: string, version: string) => Promise<void> | void;
  onCancelBuild: (pipeline: VersionSourcePipeline, buildRunId: string) => Promise<void> | void;
  onStreamLog: (
    buildRunId: string,
    onLog: (text: string) => void,
    onStatus?: (status: string) => void,
    onError?: (error: Error) => void
  ) => () => void;
  onBuildStatusChange: (buildRunId: string, status: Status) => void;
}) {
  const [sourceRef, setSourceRef] = useState(pipeline.branch);
  const [buildVersion, setBuildVersion] = useState(nextBuildSemver(pipeline.buildHistory));
  const [selectedBuildId, setSelectedBuildId] = useState(pipeline.buildHistory[0]?.id || '');
  const [buildLogsById, setBuildLogsById] = useState<Record<string, string[]>>({});
  const [logLoading, setLogLoading] = useState(false);
  const [logError, setLogError] = useState('');
  const [logKeyword, setLogKeyword] = useState('');
  const [errorsOnly, setErrorsOnly] = useState(false);
  const [showFullLog, setShowFullLog] = useState(false);
  const [paused, setPaused] = useState(false);
  const [buildSubmitting, setBuildSubmitting] = useState(false);
  const [cancelingBuildId, setCancelingBuildId] = useState('');
  const [localBuildHistory, setLocalBuildHistory] = useState(pipeline.buildHistory);
  const logContainerRef = useRef<HTMLDivElement | null>(null);
  const streamLogRef = useRef(onStreamLog);
  const buildStatusChangeRef = useRef(onBuildStatusChange);
  const selectedBuild = localBuildHistory.find((run) => run.id === selectedBuildId) || localBuildHistory[0];
  const [buildCommand, setBuildCommand] = useState(pipeline.buildCommand);
  const activeBuild = localBuildHistory.find((run) => isUnfinishedBuildStatus(run.status));
  const logLines = useMemo(() => {
    const keyword = logKeyword.trim().toLowerCase();
    const selectedLogs = selectedBuild ? buildLogsById[selectedBuild.id] : undefined;
    const sourceLines = selectedLogs?.length ? selectedLogs : pipeline.logs.length ? pipeline.logs : ['请选择一条构建记录查看日志。'];
    return sourceLines.filter((line) => {
      const normalized = line.toLowerCase();
      if (errorsOnly && !/(error|failed|失败|错误|异常)/i.test(line)) return false;
      if (keyword && !normalized.includes(keyword)) return false;
      return true;
    });
  }, [buildLogsById, errorsOnly, logKeyword, pipeline.logs, selectedBuild]);
  const visibleLogLines = showFullLog ? logLines : logLines.slice(-120);
  const hiddenLogCount = Math.max(0, logLines.length - visibleLogLines.length);

  useEffect(() => {
    streamLogRef.current = onStreamLog;
    buildStatusChangeRef.current = onBuildStatusChange;
  }, [onBuildStatusChange, onStreamLog]);

  useEffect(() => {
    setSourceRef(pipeline.branch);
    setBuildCommand(pipeline.buildCommand);
    setBuildVersion(nextBuildSemver(pipeline.buildHistory));
    setLocalBuildHistory(pipeline.buildHistory);
    setSelectedBuildId(pipeline.buildHistory[0]?.id || '');
    setBuildLogsById({});
    setLogError('');
  }, [pipeline.id]);

  useEffect(() => {
    setLocalBuildHistory(pipeline.buildHistory);
    setSelectedBuildId((current) => {
      if (current && pipeline.buildHistory.some((run) => run.id === current)) return current;
      return pipeline.buildHistory[0]?.id || '';
    });
  }, [pipeline.buildHistory]);

  useEffect(() => {
    if (!selectedBuildId) return;
    setLogLoading(true);
    setLogError('');
    const close = streamLogRef.current(selectedBuildId, (chunk) => {
      setBuildLogsById((current) => {
        const currentLines = current[selectedBuildId] || [];
        const nextLines = chunk.split(/\r?\n/).filter((line) => line.length > 0);
        return { ...current, [selectedBuildId]: [...currentLines, ...nextLines] };
      });
      setLogLoading(false);
    }, (status) => {
      setLogLoading(false);
      const nextStatus = buildStatusFromStream(status);
      if (nextStatus) buildStatusChangeRef.current(selectedBuildId, nextStatus);
      setLocalBuildHistory((current) => current.map((run) => (
        run.id === selectedBuildId ? { ...run, status: nextStatus || run.status } : run
      )));
    }, (error) => {
      setLogLoading(false);
      setLogError(error.message || '构建日志读取失败');
    });
    return close;
  }, [selectedBuildId]);

  useEffect(() => {
    if (paused) return;
    const el = logContainerRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [paused, visibleLogLines]);

  const triggerBuild = async () => {
    if (activeBuild || buildSubmitting || !sourceRef.trim() || !buildCommand.trim() || !isBuildSemver(buildVersion)) return;
    setBuildSubmitting(true);
    try {
      await onSubmit(pipeline, sourceRef, buildCommand, buildVersion);
    } finally {
      setBuildSubmitting(false);
    }
  };

  const cancelBuild = async () => {
    if (!selectedBuild || !isUnfinishedBuildStatus(selectedBuild.status)) return;
    setCancelingBuildId(selectedBuild.id);
    try {
      await onCancelBuild(pipeline, selectedBuild.id);
      onBuildStatusChange(selectedBuild.id, 'danger');
      setLocalBuildHistory((current) => current.map((run) => (
        run.id === selectedBuild.id ? { ...run, status: 'danger', duration: '已取消' } : run
      )));
    } finally {
      setCancelingBuildId('');
    }
  };

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className={DEPLOYMENT_DIALOG_CLASS}>
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <div className="dense-label">构建历史</div>
            <h2 className="mt-1 text-lg font-semibold">{pipeline.name}</h2>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭构建弹窗">
            <X className="h-4 w-4" />
          </Button>
        </div>

        <div className="flex min-h-0 flex-1 flex-col">
          <div className="border-b bg-slate-50 px-5 py-4">
            <div className="grid items-center gap-3 lg:grid-cols-[180px_150px_minmax(0,1fr)_140px]">
              <Input value={sourceRef} onChange={(event) => setSourceRef(event.target.value)} placeholder="源码引用" />
              <Input
                className="font-mono text-xs"
                value={buildVersion}
                onChange={(event) => setBuildVersion(event.target.value)}
                placeholder="版本号 v1.0.1"
              />
              <Input
                className="font-mono text-xs"
                value={buildCommand}
                onChange={(event) => setBuildCommand(event.target.value)}
                placeholder="构建命令"
              />
              <Button
                variant="outline"
                onClick={triggerBuild}
                disabled={!!activeBuild || buildSubmitting || !sourceRef.trim() || !buildCommand.trim() || !isBuildSemver(buildVersion)}
              >
                <Rocket className="h-4 w-4" />
                {buildSubmitting ? '提交中' : '触发构建'}
              </Button>
            </div>
          </div>

          <div className="grid min-h-0 flex-1 grid-cols-[320px_minmax(0,1fr)] overflow-hidden">
            <aside className="flex min-h-0 flex-col border-r bg-card">
              <div className="border-b px-4 py-3">
                <div className="text-sm font-semibold">构建时间</div>
                <div className="mt-1 text-xs text-muted-foreground">按开始时间倒序展示</div>
              </div>
              <div className="min-h-0 flex-1 space-y-2 overflow-y-auto p-4">
                {localBuildHistory.map((run, index) => (
                  <button
                    key={run.id}
                    type="button"
                    onClick={() => setSelectedBuildId(run.id)}
                    className={cn(
                      'w-full rounded-md border bg-card p-3 text-left text-sm transition-colors hover:border-primary/40',
                      selectedBuild?.id === run.id && 'border-primary bg-accent'
                    )}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <span className="font-medium">构建 {localBuildHistory.length - index}</span>
                      <BuildRunStatusBadge status={run.status} />
                    </div>
                    <div className="mt-2 text-xs text-muted-foreground">{run.startedAt} · {run.duration}</div>
                    <div className="mono mt-1 truncate text-xs text-muted-foreground">{run.branch} · {run.version}</div>
                  </button>
                ))}
                {localBuildHistory.length === 0 && (
                  <div className="rounded-md border border-dashed p-5 text-center text-sm text-muted-foreground">暂无构建记录</div>
                )}
              </div>
            </aside>

            <section className="flex min-h-0 flex-col">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
                <div>
                  <div className="text-sm font-semibold">实时日志</div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    {selectedBuild ? '构建命令' : '请选择一条构建记录查看日志'}
                    {logLoading ? ' · 日志加载中' : ''}
                  </div>
                </div>
                <div className="flex flex-wrap items-center justify-end gap-2">
                  <Input
                    value={logKeyword}
                    onChange={(event) => setLogKeyword(event.target.value)}
                    className="h-8 w-40"
                    placeholder="搜索日志"
                  />
                  <Button variant={errorsOnly ? 'default' : 'outline'} size="sm" onClick={() => setErrorsOnly((value) => !value)}>
                    只看错误
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => setShowFullLog((value) => !value)}>
                    {showFullLog ? '收起日志' : '完整日志'}
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => setPaused((value) => !value)}>
                    {paused ? '继续滚动' : '暂停滚动'}
                  </Button>
                  {selectedBuild && isUnfinishedBuildStatus(selectedBuild.status) && (
                    <Button variant="destructive" size="sm" onClick={cancelBuild} disabled={cancelingBuildId === selectedBuild.id}>
                      {cancelingBuildId === selectedBuild.id ? '取消中' : '取消构建'}
                    </Button>
                  )}
                </div>
              </div>
              <div ref={logContainerRef} className="min-h-0 flex-1 overflow-y-auto bg-slate-950 p-4">
                {logError && (
                  <div className="mb-3 rounded border border-red-900/60 bg-red-950/70 px-3 py-2 text-xs text-red-100">
                    {logError}
                  </div>
                )}
                {hiddenLogCount > 0 && (
                  <div className="mb-3 rounded border border-slate-700 bg-slate-900 px-3 py-2 text-xs text-slate-300">
                    当前仅显示最近 {visibleLogLines.length} 行，已隐藏 {hiddenLogCount} 行。
                  </div>
                )}
                {visibleLogLines.map((line, index) => (
                  <div key={`${line}-${index}`} className="mono whitespace-pre-wrap py-1 text-xs leading-5 text-slate-100">
                    {renderLogLine(line)}
                  </div>
                ))}
                {visibleLogLines.length === 0 && (
                  <div className="text-sm text-slate-400">没有匹配的日志行。</div>
                )}
              </div>
            </section>
          </div>
        </div>

        <div className="flex justify-end gap-2 border-t bg-slate-50 px-5 py-4">
          <Button variant="outline" onClick={onClose}>关闭</Button>
        </div>
      </div>
    </div>
  );
}

function BuildRunStatusBadge({ status }: { status: Status }) {
  const map: Record<Status, { label: string; className: string; dotClassName?: string }> = {
    healthy: { label: '成功', className: 'border-emerald-200 bg-emerald-50 text-emerald-700' },
    warning: { label: '不稳定', className: 'border-amber-200 bg-amber-50 text-amber-700' },
    danger: { label: '失败', className: 'border-red-200 bg-red-50 text-red-700' },
    running: {
      label: '构建中',
      className: 'border-blue-200 bg-blue-50 text-blue-700',
      dotClassName: 'build-status-dot build-status-dot-running bg-blue-500'
    },
    pending: {
      label: '等待中',
      className: 'border-slate-200 bg-slate-50 text-slate-600',
      dotClassName: 'build-status-dot build-status-dot-pending bg-slate-400'
    }
  };
  const item = map[status] || map.pending;
  return (
    <span className={cn('inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium', item.className)}>
      {item.dotClassName ? <span className={item.dotClassName} aria-hidden="true" /> : null}
      {item.label}
    </span>
  );
}

function isUnfinishedBuildStatus(status: Status) {
  return status === 'running' || status === 'pending';
}

function stageVerificationKey(stage: Pick<Stage, 'id' | 'freightId'>) {
  return `${stage.id}:${stage.freightId || ''}`;
}

function verifiedKeysFromStages(stages: Stage[]) {
  return new Set(stages
    .filter((stage) => stage.verificationStatus === 'passed' && Boolean(stage.freightId))
    .map(stageVerificationKey));
}

function pipelineStatusFromBuildHistory(history: VersionSourcePipeline['buildHistory'], fallback: Status): Status {
  return history[0]?.status || fallback;
}

function normalizeTriggeredBuildRun(run: VersionSourcePipeline['buildHistory'][number]) {
  if (!isUnfinishedBuildStatus(run.status)) return run;
  return {
    ...run,
    status: 'running' as const,
    duration: run.duration === '-' ? '进行中' : run.duration
  };
}

function mergePipelineRefresh(current: VersionSourcePipeline[], refreshed: VersionSourcePipeline[]) {
  const currentById = new Map(current.map((pipeline) => [pipeline.id, pipeline]));
  return refreshed.map((next) => mergePipelineRefreshItem(currentById.get(next.id), next));
}

function mergePipelineRefreshItem(current: VersionSourcePipeline | undefined, refreshed: VersionSourcePipeline) {
  if (!current) return refreshed;
  const activeRuns = current.buildHistory.filter((run) => isUnfinishedBuildStatus(run.status));
  if (!activeRuns.length) return refreshed;

  let changed = false;
  const nextHistory = refreshed.buildHistory.slice();
  activeRuns.forEach((activeRun) => {
    const index = nextHistory.findIndex((run) => run.id === activeRun.id);
    if (index === -1) {
      nextHistory.unshift(activeRun);
      changed = true;
      return;
    }
    if (activeRun.status === 'running' && nextHistory[index].status === 'pending') {
      nextHistory[index] = { ...nextHistory[index], status: 'running', duration: activeRun.duration || nextHistory[index].duration };
      changed = true;
    }
  });
  if (!changed) return refreshed;

  return {
    ...refreshed,
    branch: current.branch,
    buildCommand: current.buildCommand,
    buildHistory: nextHistory,
    latestVersion: latestSuccessfulBuild({ ...refreshed, buildHistory: nextHistory })?.version || '暂无版本',
    status: pipelineStatusFromBuildHistory(nextHistory, refreshed.status)
  };
}

function buildStatusFromStream(status: string): Status | undefined {
  const normalized = status.toLowerCase();
  if (['queued', 'pending', 'running', 'reconnecting', 'streaming', 'connecting'].includes(normalized)) return 'running';
  if (['succeeded', 'success', 'healthy'].includes(normalized)) return 'healthy';
  if (['failed', 'aborted', 'error', 'danger'].includes(normalized)) return 'danger';
  if (['unstable', 'warning'].includes(normalized)) return 'warning';
  return undefined;
}

function isBuildSemver(value: string) {
  return /^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(value.trim());
}

function nextBuildSemver(history: VersionSourcePipeline['buildHistory']) {
  const success = history.find((run) => run.status === 'healthy' && isBuildSemver(run.version));
  if (!success) return 'v0.0.1';
  const match = success.version.trim().match(/^(v?)(\d+)\.(\d+)\.(\d+)(?:-[0-9A-Za-z.-]+)?$/);
  if (!match) return 'v0.0.1';
  const prefix = match[1] || 'v';
  const major = Number(match[2]);
  const minor = Number(match[3]);
  const patch = Number(match[4]) + 1;
  return `${prefix}${major}.${minor}.${patch}`;
}

type LogSegment = { text: string; className?: string };

function renderLogLine(line: string): ReactNode {
  const segments = parseAnsiSegments(line);
  if (segments.some((segment) => segment.className)) {
    return segments.map((segment, index) => (
      <span key={index} className={segment.className}>{segment.text}</span>
    ));
  }
  return <span className={fallbackLogClass(line)}>{stripAnsi(line)}</span>;
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
  if (codes.includes(31) || codes.includes(91)) return 'text-red-300';
  if (codes.includes(32) || codes.includes(92)) return 'text-emerald-300';
  if (codes.includes(33) || codes.includes(93)) return 'text-yellow-300';
  if (codes.includes(34) || codes.includes(94)) return 'text-sky-300';
  if (codes.includes(35) || codes.includes(95)) return 'text-fuchsia-300';
  if (codes.includes(36) || codes.includes(96)) return 'text-cyan-300';
  return currentClass;
}

function fallbackLogClass(line: string) {
  const clean = stripAnsi(line);
  if (/(error|fail|failed|failure|失败|错误|异常)/i.test(clean)) return 'text-red-300';
  if (/(warn|warning|警告)/i.test(clean)) return 'text-yellow-300';
  if (/(success|succeeded|成功|完成)/i.test(clean)) return 'text-emerald-300';
  if (/(info|\[INFO\])/i.test(clean)) return 'text-sky-300';
  return 'text-slate-100';
}

function stripAnsi(text: string) {
  return text.replace(/\x1b\[[0-9;]*m/g, '');
}

function PipelineSourcesEditor({
  sources,
  buildEnvironmentOptions,
  onChange
}: {
  sources: PipelineSourceConfig[];
  buildEnvironmentOptions: PipelineEnvironmentOption[];
  onChange: (sources: PipelineSourceConfig[]) => void;
}) {
  function updateSource(id: string, patch: Partial<PipelineSourceConfig>) {
    onChange(sources.map((source) => source.id === id ? { ...source, ...patch } : source));
  }
  function updateSVNCheckoutPath(source: PipelineSourceConfig, pathIndex: number, patch: Partial<{ local: string; path: string; depth: string }>) {
    const paths = normalizeEditorSVNCheckoutPaths(source.svnCheckoutPaths);
    updateSource(source.id, {
      svnCheckoutPaths: paths.map((item, index) => index === pathIndex ? { ...item, ...patch } : item)
    });
  }
  function addSVNCheckoutPath(source: PipelineSourceConfig) {
    updateSource(source.id, {
      svnCheckoutPaths: [...normalizeEditorSVNCheckoutPaths(source.svnCheckoutPaths), { local: '', path: '', depth: 'infinity' }]
    });
  }
  function removeSVNCheckoutPath(source: PipelineSourceConfig, pathIndex: number) {
    const paths = normalizeEditorSVNCheckoutPaths(source.svnCheckoutPaths).filter((_, index) => index !== pathIndex);
    updateSource(source.id, { svnCheckoutPaths: paths.length ? paths : [{ local: '.', path: '', depth: 'infinity' }] });
  }

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div>
          <div className="text-sm font-semibold">代码源</div>
          <p className="mt-1 text-xs text-muted-foreground">可添加多个代码源；每条代码源独立配置分支、目录和构建命令。</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => onChange([...sources, nextPipelineSource(sources.length, buildEnvironmentOptions)])}>
          <Plus className="h-4 w-4" />
          添加代码源
        </Button>
      </div>
      <div className="space-y-4">
        {sources.map((source, index) => (
          <div key={source.id} className="rounded-lg border bg-slate-50 p-4">
            <div className="mb-4 flex items-center justify-between gap-3">
              <div className="text-sm font-semibold">{index === 0 ? '主代码源' : `代码源 ${index + 1}`}</div>
              <Button variant="ghost" size="sm" disabled={sources.length <= 1} onClick={() => onChange(sources.filter((item) => item.id !== source.id))}>
                <Trash2 className="h-4 w-4" />
                删除
              </Button>
            </div>
            <div className="space-y-3">
              <InlineField label="代码源标识">
                <Input value={source.key} onChange={(event) => updateSource(source.id, { key: event.target.value })} />
              </InlineField>
              <InlineField label="显示名称">
                <Input value={source.name} onChange={(event) => updateSource(source.id, { name: event.target.value })} />
              </InlineField>
              <InlineField label="源码类型">
                <select
                  value={source.sourceType || 'git'}
                  onChange={(event) => {
                    const nextType = event.target.value === 'svn' ? 'svn' : 'git';
                    updateSource(source.id, {
                      sourceType: nextType,
                      sourceRef: nextType === 'svn' ? (source.sourceRef || 'HEAD') : (source.sourceRef === 'HEAD' ? 'main' : source.sourceRef || source.branch || 'main'),
                      branch: nextType === 'svn' ? (source.sourceRef || 'HEAD') : (source.sourceRef === 'HEAD' ? 'main' : source.sourceRef || source.branch || 'main'),
                      svnCheckoutPaths: nextType === 'svn' ? normalizeEditorSVNCheckoutPaths(source.svnCheckoutPaths) : source.svnCheckoutPaths
                    });
                  }}
                  className="h-10 w-full rounded-md border bg-card px-3 text-sm"
                >
                  <option value="git">Git</option>
                  <option value="svn">SVN</option>
                </select>
              </InlineField>
              <InlineField label={source.sourceType === 'svn' ? 'SVN 地址' : 'Git 仓库地址'}>
                <Input
                  value={source.sourceUrl || source.repository}
                  onChange={(event) => updateSource(source.id, { repository: event.target.value, sourceUrl: event.target.value })}
                  placeholder={source.sourceType === 'svn' ? 'svn://repo.company.com/project/trunk/app' : 'https://gitlab.example/group/project.git'}
                />
              </InlineField>
              {(source.sourceType || 'git') === 'git' ? (
                <InlineField label="分支">
                  <Input
                    value={source.sourceRef || source.branch}
                    onChange={(event) => updateSource(source.id, { sourceRef: event.target.value, branch: event.target.value })}
                    placeholder="例如：main、release/1.0"
                  />
                </InlineField>
              ) : (
                <>
                  <InlineField label="SVN revision">
                    <Input
                      value={source.svnRevision || ''}
                      onChange={(event) => updateSource(source.id, { svnRevision: event.target.value })}
                      placeholder="可选，默认 HEAD"
                    />
                  </InlineField>
                  <div className="rounded-md border bg-card p-3">
                    <div className="mb-3 flex items-center justify-between gap-3">
                      <div>
                        <div className="text-xs font-semibold text-slate-700">SVN 检出目录</div>
                        <div className="mt-1 text-[11px] text-muted-foreground">基于 SVN 地址拼接相对子路径，可用于只检出根文件和指定模块。</div>
                      </div>
                      <Button variant="outline" size="sm" onClick={() => addSVNCheckoutPath(source)}>
                        <Plus className="h-4 w-4" />
                        添加目录
                      </Button>
                    </div>
                    <div className="space-y-2">
                      {normalizeEditorSVNCheckoutPaths(source.svnCheckoutPaths).map((item, pathIndex) => (
                        <div key={`${source.id}-svn-path-${pathIndex}`} className="grid gap-2 md:grid-cols-[1fr_1fr_140px_72px]">
                          <Input
                            value={item.local}
                            onChange={(event) => updateSVNCheckoutPath(source, pathIndex, { local: event.target.value })}
                            placeholder="本地目录，如 . 或 web/server"
                          />
                          <Input
                            value={item.path}
                            onChange={(event) => updateSVNCheckoutPath(source, pathIndex, { path: event.target.value })}
                            placeholder="SVN 子路径，根目录留空"
                          />
                          <select
                            value={item.depth || 'infinity'}
                            onChange={(event) => updateSVNCheckoutPath(source, pathIndex, { depth: event.target.value })}
                            className="h-10 rounded-md border bg-card px-3 text-sm"
                          >
                            <option value="empty">empty</option>
                            <option value="files">files</option>
                            <option value="immediates">immediates</option>
                            <option value="infinity">infinity</option>
                          </select>
                          <Button variant="ghost" size="sm" disabled={normalizeEditorSVNCheckoutPaths(source.svnCheckoutPaths).length <= 1} onClick={() => removeSVNCheckoutPath(source, pathIndex)}>
                            删除
                          </Button>
                        </div>
                      ))}
                    </div>
                  </div>
                </>
              )}
              <InlineField label="源码子目录 source_path">
                <Input value={source.sourcePath} onChange={(event) => updateSource(source.id, { sourcePath: event.target.value })} />
              </InlineField>
              <InlineField label="构建环境">
                <select
                  value={source.buildEnvironmentId || optionByName(buildEnvironmentOptions, source.buildEnvironment)?.id || ''}
                  onChange={(event) => {
                    const option = optionById(buildEnvironmentOptions, event.target.value);
                    updateSource(source.id, {
                      buildEnvironmentId: option?.id || event.target.value,
                      buildEnvironment: option?.name || event.target.value
                    });
                  }}
                  className="h-10 w-full rounded-md border bg-card px-3 text-sm"
                >
                  {buildEnvironmentOptions.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}
                </select>
              </InlineField>
              <InlineField label="构建命令 build_command">
                <textarea value={source.buildCommand} onChange={(event) => updateSource(source.id, { buildCommand: event.target.value })} className="min-h-24 w-full rounded-md border bg-card px-3 py-2 font-mono text-xs" />
              </InlineField>
              <InlineField label="产物拷贝命令 artifact_copy_command">
                <textarea value={source.artifactCopyCommand} onChange={(event) => updateSource(source.id, { artifactCopyCommand: event.target.value })} className="min-h-24 w-full rounded-md border bg-card px-3 py-2 font-mono text-xs" />
              </InlineField>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function normalizeEditorSVNCheckoutPaths(paths?: Array<{ local: string; path: string; depth: string }>) {
  const cleaned = (paths || [])
    .map((item) => ({
      local: (item.local || '.').trim() || '.',
      path: (item.path || '').trim(),
      depth: (item.depth || 'infinity').trim() || 'infinity'
    }))
    .filter((item) => item.local || item.path);
  return cleaned.length ? cleaned : [{ local: '.', path: '', depth: 'infinity' }];
}

function ConfigSection({
  icon,
  title,
  description,
  action,
  children
}: {
  icon: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="bg-white">
      <div className="flex items-start justify-between gap-4 border-b bg-slate-50 px-5 py-3">
        <div className="flex min-w-0 gap-3">
          <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded border bg-white text-primary">
            {icon}
          </div>
          <div className="min-w-0">
            <h3 className="truncate text-sm font-semibold">{title}</h3>
            {description && <p className="mt-1 text-xs text-muted-foreground">{description}</p>}
          </div>
        </div>
        {action && <div className="shrink-0">{action}</div>}
      </div>
      <div className="p-5">{children}</div>
    </section>
  );
}

function CompactField({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <label className="grid min-h-9 items-start gap-3 text-sm sm:grid-cols-[132px_minmax(0,1fr)]">
      <span className="pt-2 text-xs font-medium text-slate-600">{label}</span>
      <div className="min-w-0">{children}</div>
    </label>
  );
}

function NumberTextInput({
  value,
  min,
  max,
  defaultValue,
  onValueChange,
  className,
  disabled
}: {
  value?: number;
  min?: number;
  max?: number;
  defaultValue?: number;
  onValueChange: (value: number) => void;
  className?: string;
  disabled?: boolean;
}) {
  const [draft, setDraft] = useState(value === undefined || Number.isNaN(value) ? '' : String(value));

  useEffect(() => {
    const next = value === undefined || Number.isNaN(value) ? '' : String(value);
    setDraft((current) => (current === next ? current : next));
  }, [value]);

  function commit(raw: string) {
    if (raw === '') return;
    const parsed = Number(raw);
    if (Number.isNaN(parsed)) return;
    const clamped = Math.min(max ?? parsed, Math.max(min ?? parsed, parsed));
    onValueChange(clamped);
  }

  return (
    <Input
      value={draft}
      inputMode="numeric"
      pattern="[0-9]*"
      className={cn('min-w-24 font-mono tabular-nums', className)}
      disabled={disabled}
      onChange={(event) => {
        const raw = event.target.value;
        if (!/^\d*$/.test(raw)) return;
        setDraft(raw);
        commit(raw);
      }}
      onBlur={() => {
        if (draft !== '') return;
        const fallback = defaultValue ?? min ?? 0;
        setDraft(String(fallback));
        onValueChange(fallback);
      }}
    />
  );
}

function ContainerProbeFields({
  title,
  probe,
  onChange
}: {
  title: string;
  probe: WorkloadProbeConfig;
  onChange: (patch: Partial<WorkloadProbeConfig>) => void;
}) {
  const probeType = probe.probeType || 'http';
  return (
    <div className="rounded-md border bg-white p-3">
      <div className="mb-3 border-b pb-2 text-xs font-semibold text-slate-700">{title}</div>
      <div className="grid gap-3">
        <CompactField label="探针类型">
          <select
            value={probeType}
            onChange={(event) => onChange({ probeType: event.target.value as 'http' | 'tcp' })}
            className="h-9 w-full rounded-md border bg-card px-3 text-sm"
          >
            <option value="http">HTTP 探针</option>
            <option value="tcp">TCP 探针</option>
          </select>
        </CompactField>
        {probeType === 'http' ? (
          <CompactField label="路径">
            <Input value={probe.path || ''} onChange={(event) => onChange({ path: event.target.value })} placeholder="/healthz" />
          </CompactField>
        ) : null}
        <CompactField label="端口">
          <NumberTextInput value={probe.port} min={1} max={65535} defaultValue={8080} onValueChange={(value) => onChange({ port: value })} />
        </CompactField>
        <div className="grid gap-3">
          <CompactField label="初始等待">
            <NumberTextInput value={probe.initialDelaySeconds ?? 0} min={0} defaultValue={0} onValueChange={(value) => onChange({ initialDelaySeconds: value })} />
          </CompactField>
          <CompactField label="周期">
            <NumberTextInput value={probe.periodSeconds ?? 10} min={1} defaultValue={10} onValueChange={(value) => onChange({ periodSeconds: value })} />
          </CompactField>
          <CompactField label="超时">
            <NumberTextInput value={probe.timeoutSeconds ?? 1} min={1} defaultValue={1} onValueChange={(value) => onChange({ timeoutSeconds: value })} />
          </CompactField>
          <CompactField label="失败阈值">
            <NumberTextInput value={probe.failureThreshold ?? 3} min={1} defaultValue={3} onValueChange={(value) => onChange({ failureThreshold: value })} />
          </CompactField>
        </div>
      </div>
    </div>
  );
}

function defaultContainerProbeConfig(port = 8080) {
  return {
    livenessProbe: defaultProbeForKey('livenessProbe', port),
    readinessProbe: defaultProbeForKey('readinessProbe', port),
    startupProbe: defaultProbeForKey('startupProbe', port)
  };
}

function defaultProbeForKey(probeKey: 'livenessProbe' | 'readinessProbe' | 'startupProbe', port = 8080): WorkloadProbeConfig {
  if (probeKey === 'readinessProbe') {
    return {
      enabled: true,
      probeType: 'http',
      path: '/healthz',
      port,
      initialDelaySeconds: 10,
      periodSeconds: 10,
      timeoutSeconds: 1,
      failureThreshold: 5,
      successThreshold: 1
    };
  }
  if (probeKey === 'startupProbe') {
    return {
      enabled: false,
      probeType: 'http',
      path: '/healthz',
      port,
      initialDelaySeconds: 0,
      periodSeconds: 10,
      timeoutSeconds: 1,
      failureThreshold: 30,
      successThreshold: 1
    };
  }
  return {
    enabled: true,
    probeType: 'http',
    path: '/healthz',
    port,
    initialDelaySeconds: 20,
    periodSeconds: 10,
    timeoutSeconds: 1,
    failureThreshold: 3,
    successThreshold: 1
  };
}

function cloneVersionSourceConfig(config: VersionSourceConfig): VersionSourceConfig {
  return {
    ...config,
    workloads: config.workloads.map((workload) => ({
      ...workload,
      containers: workload.containers.map((container) => ({
        ...container,
        imageSource: { ...container.imageSource },
        livenessProbe: container.livenessProbe ? { ...container.livenessProbe } : undefined,
        readinessProbe: container.readinessProbe ? { ...container.readinessProbe } : undefined,
        startupProbe: container.startupProbe ? { ...container.startupProbe } : undefined
      }))
    }))
  };
}

function InlineField({ label, children, compact }: { label: string; children: ReactNode; compact?: boolean }) {
  return (
    <label className={cn(
      'grid items-start gap-3 text-sm',
      compact ? 'grid-cols-[72px_minmax(0,1fr)]' : 'sm:grid-cols-[150px_minmax(0,1fr)]'
    )}>
      <span className="pt-2 text-xs font-medium text-muted-foreground">{label}</span>
      <div className="min-w-0">{children}</div>
    </label>
  );
}

function ConfigList({
  title,
  addLabel,
  emptyText,
  items,
  onAdd,
  onRemove,
  render
}: {
  title: string;
  addLabel: string;
  emptyText: string;
  items: unknown[];
  onAdd: () => void;
  onRemove: (index: number) => void;
  render: (item: Record<string, unknown>, index: number) => ReactNode;
}) {
  return (
    <div className="mt-4 first:mt-0">
      <div className="mb-2 flex items-center justify-between gap-3">
        <div className="text-xs font-semibold text-slate-700">{title}</div>
        <Button size="sm" variant="outline" onClick={onAdd}>
          <Plus className="h-3.5 w-3.5" />
          {addLabel}
        </Button>
      </div>
      <div className="space-y-2">
        {items.length === 0 && (
          <div className="rounded border border-dashed bg-slate-50 px-3 py-2 text-xs text-muted-foreground">{emptyText}</div>
        )}
        {items.map((item, index) => (
          <div key={(item as { id?: string }).id || index} className="grid gap-2 rounded border bg-white p-3">
            <div className="flex items-center justify-between border-b pb-2">
              <span className="text-xs font-medium text-muted-foreground">第 {index + 1} 项</span>
              <Button variant="ghost" size="sm" onClick={() => onRemove(index)}>
                <Trash2 className="h-3.5 w-3.5" />
                删除
              </Button>
            </div>
            <div className="grid gap-2">{render(item as Record<string, unknown>, index)}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

type RuntimePodInfo = Stage['workloads'][number]['podDetails'][number] & {
  resourceId?: string;
  namespace?: string;
  containerName?: string;
};

type RuntimeWorkloadInfo = Stage['workloads'][number] & {
  resourceId?: string;
  namespace?: string;
  podDetails: RuntimePodInfo[];
};

function RuntimeDrawer({
  applicationId,
  stage,
  freightDisplayName,
  onClose,
  onReload
}: {
  applicationId: string;
  stage: Stage;
  freightDisplayName: string;
  onClose: () => void;
  onReload: () => Promise<void>;
}) {
  const [actionMessage, setActionMessage] = useState('');
  const [actionTarget, setActionTarget] = useState('');

  async function runRuntimeAction(target: string, action: () => Promise<unknown>, successText: string) {
    setActionTarget(target);
    setActionMessage('');
    try {
      await action();
      setActionMessage(successText);
      await onReload();
    } catch (error) {
      setActionMessage(error instanceof Error ? error.message : '运行态操作失败');
    } finally {
      setActionTarget('');
    }
  }

  async function handleRestart(resourceId: string | undefined, label: string) {
    if (!resourceId) {
      setActionMessage('当前资源缺少后端 resourceId，无法执行重启');
      return;
    }
    await runRuntimeAction(
      `restart:${resourceId}`,
      () => restartRuntimeResource(applicationId, stage.key, resourceId),
      `${label} 重启请求已发送`
    );
  }

  async function handleLogs(pod: RuntimePodInfo) {
    if (!pod.resourceId) {
      setActionMessage('当前 Pod 缺少后端 resourceId，无法打开日志');
      return;
    }
    await runRuntimeAction(
      `logs:${pod.resourceId}`,
      () => checkRuntimePodLogs(applicationId, stage.key, pod.resourceId || '', pod.containerName),
      `已确认 ${pod.name} 支持日志流，下一步可接入实时日志面板`
    );
  }

  async function handleTerminal(pod: RuntimePodInfo) {
    if (!pod.resourceId) {
      setActionMessage('当前 Pod 缺少后端 resourceId，无法打开终端');
      return;
    }
    await runRuntimeAction(
      `terminal:${pod.resourceId}`,
      () => checkRuntimePodTerminal(applicationId, stage.key, pod.resourceId || '', pod.containerName),
      `已确认 ${pod.name} 支持终端，下一步可接入交互终端面板`
    );
  }

  return (
    <aside className="fixed inset-y-0 right-0 z-40 flex w-full max-w-[520px] flex-col border-l bg-card shadow-xl">
      <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
        <div>
          <div className="dense-label">Stage 运行态</div>
          <h2 className="mt-1 text-lg font-semibold">{stage.name}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{stage.cluster} · {stage.namespace}</p>
        </div>
        <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭运行态抽屉">
          <X className="h-4 w-4" />
        </Button>
      </div>
      <div className="flex-1 space-y-4 overflow-y-auto p-5">
        <div className="grid grid-cols-2 gap-3">
          <Metric label="同步状态" value={stage.sync} />
          <Metric label="版本" value={freightDisplayName} mono />
          <Metric label="绑定集群" value={stage.clusterBindingId} mono />
          <Metric label="进度" value={`${stage.progress}%`} mono />
        </div>
        <div className="space-y-2">
          <h3 className="text-sm font-semibold">Workload 实时状态</h3>
          {actionMessage && (
            <div className={cn(
              'rounded-md border px-3 py-2 text-xs',
              actionMessage.includes('失败') || actionMessage.includes('无法')
                ? 'border-red-200 bg-red-50 text-red-700'
                : 'border-emerald-200 bg-emerald-50 text-emerald-700'
            )}>
              {actionMessage}
            </div>
          )}
          {stage.workloads.map((workload) => (
            <RuntimeWorkloadTree
              key={workload.name}
              workload={workload as RuntimeWorkloadInfo}
              actionTarget={actionTarget}
              onRestart={handleRestart}
              onOpenLogs={handleLogs}
              onOpenTerminal={handleTerminal}
            />
          ))}
        </div>
      </div>
    </aside>
  );
}

function RuntimeWorkloadTree({
  workload,
  actionTarget,
  onRestart,
  onOpenLogs,
  onOpenTerminal
}: {
  workload: RuntimeWorkloadInfo;
  actionTarget: string;
  onRestart: (resourceId: string | undefined, label: string) => void;
  onOpenLogs: (pod: RuntimePodInfo) => void;
  onOpenTerminal: (pod: RuntimePodInfo) => void;
}) {
  return (
    <div className="rounded-md border bg-card">
      <div className="flex items-start justify-between gap-3 border-b bg-slate-50 px-3 py-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2 font-medium">
            <Box className="h-4 w-4 text-primary" />
            {workload.displayName}
          </div>
          <div className="mono mt-1 text-xs text-muted-foreground">{workload.name}</div>
        </div>
        <StatusBadge status={workload.status} />
      </div>

      <div className="p-3">
        <div className="rounded-md border bg-background">
          <div className="flex items-center justify-between gap-3 border-b px-3 py-2 text-sm">
            <div className="flex min-w-0 items-center gap-2">
              <ChevronRight className="h-4 w-4 text-muted-foreground" />
              <Server className="h-4 w-4 text-primary" />
              <span className="font-medium">{workloadKindLabel(workload.kind)}</span>
              <span className="mono min-w-0 truncate text-xs text-muted-foreground" title={workload.image}>
                {workload.image}
              </span>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <span className="mono text-xs text-muted-foreground">{workload.replicas}</span>
              <Button
                variant="outline"
                size="sm"
                disabled={actionTarget === `restart:${workload.resourceId || ''}`}
                onClick={() => onRestart(workload.resourceId, workload.displayName)}
              >
                <RotateCcw className="h-3.5 w-3.5" />
                重启
              </Button>
            </div>
          </div>

          <div className="divide-y">
            {workload.podDetails.map((pod) => (
              <div key={pod.name} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-3 py-2.5 text-xs">
                <div className="flex min-w-0 items-center gap-2">
                  <span className={cn('h-2 w-2 shrink-0 rounded-full', pod.status === 'healthy' ? 'bg-success' : pod.status === 'warning' ? 'bg-warning' : 'bg-danger')} />
                  <span className="mono min-w-0 flex-1 truncate font-medium text-foreground" title={pod.name}>
                    {pod.name}
                  </span>
                  <div className="shrink-0">
                    <StatusBadge status={pod.status} />
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2"
                    disabled={actionTarget === `logs:${pod.resourceId || ''}`}
                    onClick={() => onOpenLogs(pod as RuntimePodInfo)}
                  >
                    <FileText className="h-3.5 w-3.5" />
                    日志
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2"
                    disabled={actionTarget === `terminal:${pod.resourceId || ''}`}
                    onClick={() => onOpenTerminal(pod as RuntimePodInfo)}
                  >
                    <Terminal className="h-3.5 w-3.5" />
                    终端
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2"
                    disabled={actionTarget === `restart:${pod.resourceId || ''}`}
                    onClick={() => onRestart(pod.resourceId, pod.name)}
                  >
                    <RotateCcw className="h-3.5 w-3.5" />
                    重启
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

type StageOverrideValues = {
  replicas?: string;
  containers: Record<string, {
    cpu?: string;
    memory?: string;
    limitCpu?: string;
    limitMemory?: string;
    envVars?: Array<{ name: string; value: string }>;
    secretRefs?: Array<{ name: string; secretRef: string }>;
    configFiles?: Array<{ mountPath: string; content: string }>;
  }>;
  ingressHosts?: Array<{ host?: string; path?: string; rewrite?: boolean; rewritePath?: string }>;
  ingressTls?: boolean;
  ingressTlsRedirect?: boolean;
  ingressRewrite?: boolean;
  ingressRewritePath?: string;
};

function StageConfigDialog({
  applicationId,
  stage,
  workloads,
  onClose,
  onSaved
}: {
  applicationId: string;
  stage: Stage;
  workloads: VersionSourceWorkloadConfig[];
  onClose: () => void;
  onSaved: () => void | Promise<void>;
}) {
  const [activeWorkloadId, setActiveWorkloadId] = useState(workloads[0]?.id || '');
  const [overrides, setOverrides] = useState<Record<string, StageOverrideValues>>({});
  const [defaults, setDefaults] = useState<Record<string, VersionSourceWorkloadConfig>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const activeWorkload = workloads.find((w) => w.id === activeWorkloadId) || workloads[0];

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    const defaultsMap: Record<string, VersionSourceWorkloadConfig> = {};
    workloads.forEach((w) => { defaultsMap[w.id] = w; });
    setDefaults(defaultsMap);

    Promise.all(workloads.map(async (w) => {
      const cfg = await loadWorkloadStageConfig(applicationId, w.id, stage.key);
      return { id: w.id, cfg };
    })).then((results) => {
      if (cancelled) return;
      const map: Record<string, StageOverrideValues> = {};
      results.forEach(({ id, cfg }) => {
        const wl = defaultsMap[id];
        if (!cfg) {
          // No override exists — pre-fill from version source defaults
          const containers: StageOverrideValues['containers'] = {};
          if (wl) {
            wl.containers.forEach((c) => {
              containers[c.name] = {
                cpu: c.cpu || undefined,
                memory: c.memory || undefined,
                limitCpu: c.limitCpu || undefined,
                limitMemory: c.limitMemory || undefined,
                envVars: (c.envVars || []).filter((e) => e.name).map((e) => ({ name: e.name, value: e.value || '' })),
                secretRefs: (c.secretRefs || []).filter((s) => s.name).map((s) => ({ name: s.name, secretRef: s.secretRef || '' })),
                configFiles: (c.configFiles || []).filter((f) => f.mountPath).map((f) => ({ mountPath: f.mountPath, content: f.content || '' })),
              };
            });
          }
          map[id] = {
            replicas: wl?.replicas ? String(wl.replicas) : undefined,
            containers,
            ingressHosts: wl?.enableDomainAccess && wl?.domain ? [{ host: wl.domain, path: wl.ingressPath || '/' }] : undefined,
            ingressTls: wl?.ingressTls || false,
            ingressTlsRedirect: wl?.ingressTlsRedirect || false,
            ingressRewrite: wl?.ingressRewrite || false,
            ingressRewritePath: wl?.ingressRewritePath || '/',
          };
        } else {
          // Override exists — parse from response (top-level fields + values_override.containers)
          const containers: StageOverrideValues['containers'] = {};
          const valContainers = cfg.values_override?.containers || cfg.valuesOverride?.containers || [];
          if (valContainers.length) {
            valContainers.forEach((c: any) => {
              const name = c.name || 'app';
              containers[name] = {
                cpu: c.cpu || undefined,
                memory: c.memory || undefined,
                limitCpu: c.limit_cpu || c.limitCpu || undefined,
                limitMemory: c.limit_memory || c.limitMemory || undefined,
                envVars: normalizeKV(c.env_vars || c.envVars),
                secretRefs: normalizeSecretRefsCfg(c.secret_refs || c.secretRefs),
                configFiles: normalizeConfigFilesCfg(c.config_files || c.configFiles),
              };
            });
          } else if (wl?.containers?.[0]) {
            // Fallback: read top-level fields as first container's values
            const firstName = wl.containers[0].name;
            const rr = cfg.resource_requests || cfg.resourceRequests;
            const rl = cfg.resource_limits || cfg.resourceLimits;
            containers[firstName] = {
              cpu: rr?.cpu || undefined,
              memory: rr?.memory || undefined,
              limitCpu: rl?.cpu || undefined,
              limitMemory: rl?.memory || undefined,
              envVars: normalizeKV(cfg.env_vars || cfg.envVars),
              secretRefs: normalizeSecretRefsCfg(cfg.secret_refs || cfg.secretRefs),
              configFiles: normalizeConfigFilesCfg(cfg.config_files || cfg.configFiles),
            };
          }
          // Merge version source containers that are missing from the override
          if (wl) {
            wl.containers.forEach((c) => {
              if (!containers[c.name]) {
                containers[c.name] = {
                  cpu: c.cpu || undefined,
                  memory: c.memory || undefined,
                  limitCpu: c.limitCpu || undefined,
                  limitMemory: c.limitMemory || undefined,
                  envVars: (c.envVars || []).filter((e) => e.name).map((e) => ({ name: e.name, value: e.value || '' })),
                  secretRefs: (c.secretRefs || []).filter((s) => s.name).map((s) => ({ name: s.name, secretRef: s.secretRef || '' })),
                  configFiles: (c.configFiles || []).filter((f) => f.mountPath).map((f) => ({ mountPath: f.mountPath, content: f.content || '' })),
                };
              } else {
                // For existing containers, fill in env vars from version source if override has none
                const existing = containers[c.name];
                if (!existing.envVars?.length && c.envVars?.length) {
                  existing.envVars = c.envVars.filter((e) => e.name).map((e) => ({ name: e.name, value: e.value || '' }));
                }
                if (!existing.secretRefs?.length && c.secretRefs?.length) {
                  existing.secretRefs = c.secretRefs.filter((s) => s.name).map((s) => ({ name: s.name, secretRef: s.secretRef || '' }));
                }
                if (!existing.configFiles?.length && c.configFiles?.length) {
                  existing.configFiles = c.configFiles.filter((f) => f.mountPath).map((f) => ({ mountPath: f.mountPath, content: f.content || '' }));
                }
              }
            });
          }
          const ingress = cfg.ingress_hosts || cfg.ingressHosts || [];
          const firstIngress = ingress[0] as any;
          map[id] = {
            replicas: cfg.replicas ? String(cfg.replicas) : undefined,
            containers,
            ingressHosts: ingress.length ? ingress.map((h: any) => ({
              host: h.host || h.server_name || h.serverName || '',
              path: h.path || '/',
              rewrite: !!h.rewrite,
              rewritePath: h.rewrite_path || h.rewritePath || '/'
            })) : undefined,
            ingressTls: firstIngress?.tls ?? false,
            ingressTlsRedirect: firstIngress?.tls_redirect ?? firstIngress?.tlsRedirect ?? false,
            ingressRewrite: firstIngress?.rewrite ?? false,
            ingressRewritePath: firstIngress?.rewrite_path || firstIngress?.rewritePath || '/',
          };
        }
      });
      setOverrides(map);
      setLoading(false);
    }).catch(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, [applicationId, stage.key]);

  function updateOverride(workloadId: string, patch: Partial<StageOverrideValues>) {
    setOverrides((prev) => {
      const current = prev[workloadId] || { containers: {} };
      return { ...prev, [workloadId]: { ...current, ...patch, containers: patch.containers ?? current.containers ?? {} } };
    });
  }

  function updateContainerOverride(workloadId: string, containerName: string, patch: Partial<StageOverrideValues['containers'][string]>) {
    setOverrides((prev) => {
      const current = prev[workloadId] || { containers: {} };
      return { ...prev, [workloadId]: { ...current, containers: { ...current.containers, [containerName]: { ...current.containers[containerName], ...patch } } } };
    });
  }

  async function handleSave() {
    setSaving(true);
    try {
      await Promise.all(workloads.map(async (wl) => {
        const ov = overrides[wl.id];
        if (!ov) return;
        const payload: any = {};
        if (ov.replicas) payload.replicas = Number(ov.replicas);
        if (ov.ingressHosts?.length) {
          payload.ingress_hosts = ov.ingressHosts.map((h) => ({
            host: h.host || '',
            path: h.path || '/',
            tls: !!ov.ingressTls,
            tls_redirect: !!ov.ingressTlsRedirect,
            rewrite: !!ov.ingressRewrite,
            rewrite_path: ov.ingressRewritePath || '/'
          }));
        }
        // Flatten first container to top-level resource fields for backend compat
        const firstContainer = wl.containers[0];
        const firstCo = firstContainer ? ov.containers[firstContainer.name] : undefined;
        if (firstCo) {
          if (firstCo.cpu || firstCo.memory) payload.resource_requests = { cpu: firstCo.cpu || '', memory: firstCo.memory || '' };
          if (firstCo.limitCpu || firstCo.limitMemory) payload.resource_limits = { cpu: firstCo.limitCpu || '', memory: firstCo.limitMemory || '' };
          if (firstCo.envVars?.length) payload.env_vars = firstCo.envVars.filter((e) => e.name);
          if (firstCo.secretRefs?.length) payload.secret_refs = firstCo.secretRefs.filter((s) => s.name).map((s) => ({ name: s.name, secret_ref: s.secretRef }));
          if (firstCo.configFiles?.length) payload.config_files = firstCo.configFiles.filter((f) => f.mountPath).map((f) => ({ mount_path: f.mountPath, content: f.content }));
        }
        // Also store all containers in values_override for multi-container support
        const containers: any[] = [];
        wl.containers.forEach((c) => {
          const co = ov.containers[c.name];
          if (!co) return;
          containers.push({
            name: c.name,
            cpu: co.cpu || '', memory: co.memory || '',
            limit_cpu: co.limitCpu || '', limit_memory: co.limitMemory || '',
            env_vars: (co.envVars || []).filter((e) => e.name),
            secret_refs: (co.secretRefs || []).filter((s) => s.name).map((s) => ({ name: s.name, secret_ref: s.secretRef })),
            config_files: (co.configFiles || []).filter((f) => f.mountPath).map((f) => ({ mount_path: f.mountPath, content: f.content }))
          });
        });
        if (containers.length) payload.values_override = { containers };
        await saveWorkloadStageConfig(applicationId, wl.id, stage.key, payload);
      }));
      await onSaved();
      onClose();
    } catch (err) {
      console.error('保存 Stage 配置失败', err);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-slate-950/40 p-4">
      <div className={DEPLOYMENT_DIALOG_CLASS}>
        <div className="flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <div className="dense-label">Stage 配置覆盖</div>
            <h2 className="mt-1 text-lg font-semibold">{stage.name}</h2>
            <p className="mt-1 text-sm text-muted-foreground">未填写的字段将继承版本源默认值（灰色 placeholder 所示）</p>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="关闭配置弹窗">
            <X className="h-4 w-4" />
          </Button>
        </div>

        <div className="grid min-h-0 flex-1 grid-cols-[248px_minmax(0,1fr)] overflow-hidden">
          <aside className="border-r bg-slate-50 p-4">
            <div className="mb-3 text-sm font-semibold">工作负载</div>
            <div className="space-y-2">
              {workloads.map((w) => (
                <button
                  key={w.id}
                  type="button"
                  onClick={() => setActiveWorkloadId(w.id)}
                  className={cn(
                    'w-full rounded-md border bg-card p-3 text-left transition-colors hover:border-primary/40',
                    activeWorkloadId === w.id && 'border-primary bg-accent'
                  )}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0 space-y-1">
                      <div className="truncate text-sm font-semibold">{w.name}</div>
                      <div className="flex flex-wrap gap-1">
                        <Badge variant="outline">{workloadKindLabel(w.kind)}</Badge>
                        <Badge variant="secondary">{w.containers.length} 容器</Badge>
                      </div>
                    </div>
                    <Layers3 className="h-4 w-4 text-muted-foreground" />
                  </div>
                </button>
              ))}
            </div>
          </aside>

          <div className="min-h-0 overflow-y-auto bg-white">
            {loading ? (
              <div className="flex min-h-[200px] items-center justify-center text-sm text-muted-foreground">加载配置中...</div>
            ) : activeWorkload ? (
              <StageConfigWorkloadForm
                workload={activeWorkload}
                override={overrides[activeWorkload.id] || { containers: {} }}
                defaults={defaults[activeWorkload.id]}
                onUpdateOverride={(patch) => updateOverride(activeWorkload.id, patch)}
                onUpdateContainer={(name, patch) => updateContainerOverride(activeWorkload.id, name, patch)}
              />
            ) : (
              <div className="flex min-h-[200px] items-center justify-center text-sm text-muted-foreground">暂无工作负载</div>
            )}
          </div>
        </div>

        <div className="flex justify-end gap-2 border-t bg-slate-50 px-5 py-4">
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button onClick={handleSave} disabled={saving}>
            <CheckCircle2 className="h-4 w-4" />
            {saving ? '保存中...' : '保存覆盖配置'}
          </Button>
        </div>
      </div>
    </div>
  );
}

function StageConfigWorkloadForm({
  workload,
  override,
  defaults,
  onUpdateOverride,
  onUpdateContainer
}: {
  workload: VersionSourceWorkloadConfig;
  override: StageOverrideValues;
  defaults?: VersionSourceWorkloadConfig;
  onUpdateOverride: (patch: Partial<StageOverrideValues>) => void;
  onUpdateContainer: (containerName: string, patch: Partial<StageOverrideValues['containers'][string]>) => void;
}) {
  return (
    <div className="divide-y">
      <ConfigSection icon={<Server className="h-4 w-4" />} title="控制器" description="副本数配置">
        <div className="grid gap-3">
          <CompactField label="副本数">
            <Input
              type="number"
              min={0}
              value={override.replicas ?? ''}
              placeholder={String(defaults?.replicas ?? 1)}
              onChange={(e) => onUpdateOverride({ replicas: e.target.value || undefined })}
            />
          </CompactField>
        </div>
      </ConfigSection>

      {workload.containers.map((container) => {
        const co = override.containers[container.name] || {};
        return (
          <ConfigSection
            key={container.name}
            icon={<Box className="h-4 w-4" />}
            title={`容器: ${container.name}`}
            description="资源与环境变量覆盖"
          >
            <div className="grid gap-3">
              <div className="grid grid-cols-2 gap-3">
                <CompactField label="CPU Request">
                  <Input value={co.cpu ?? ''} placeholder={container.cpu || '250m'} onChange={(e) => onUpdateContainer(container.name, { cpu: e.target.value || undefined })} />
                </CompactField>
                <CompactField label="Memory Request">
                  <Input value={co.memory ?? ''} placeholder={container.memory || '256Mi'} onChange={(e) => onUpdateContainer(container.name, { memory: e.target.value || undefined })} />
                </CompactField>
                <CompactField label="CPU Limit">
                  <Input value={co.limitCpu ?? ''} placeholder={container.limitCpu || '未设置'} onChange={(e) => onUpdateContainer(container.name, { limitCpu: e.target.value || undefined })} />
                </CompactField>
                <CompactField label="Memory Limit">
                  <Input value={co.limitMemory ?? ''} placeholder={container.limitMemory || '未设置'} onChange={(e) => onUpdateContainer(container.name, { limitMemory: e.target.value || undefined })} />
                </CompactField>
              </div>

              <StageEnvVarsEditor
                envVars={co.envVars || []}
                defaultEnvVars={container.envVars}
                onChange={(envVars) => onUpdateContainer(container.name, { envVars })}
              />

              <StageSecretRefsEditor
                secretRefs={co.secretRefs || []}
                defaultSecretRefs={container.secretRefs}
                onChange={(secretRefs) => onUpdateContainer(container.name, { secretRefs })}
              />

              <StageConfigFilesEditor
                configFiles={co.configFiles || []}
                defaultConfigFiles={container.configFiles}
                onChange={(configFiles) => onUpdateContainer(container.name, { configFiles })}
              />
            </div>
          </ConfigSection>
        );
      })}

      {workload.enableDomainAccess && (
        <ConfigSection icon={<Globe2 className="h-4 w-4" />} title="访问入口" description="域名覆盖">
          <div className="grid gap-3">
            <CompactField label="域名">
              <Input
                value={override.ingressHosts?.[0]?.host ?? ''}
                placeholder={defaults?.domain || '未设置'}
                onChange={(e) => onUpdateOverride({ ingressHosts: [{ ...(override.ingressHosts?.[0] || {}), host: e.target.value }] })}
              />
            </CompactField>
            <CompactField label="路径">
              <Input
                value={override.ingressHosts?.[0]?.path ?? ''}
                placeholder={defaults?.ingressPath || '/'}
                onChange={(e) => onUpdateOverride({ ingressHosts: [{ ...(override.ingressHosts?.[0] || {}), path: e.target.value }] })}
              />
            </CompactField>
            <label className="flex h-9 items-center gap-2 rounded-md border bg-card px-3 text-sm">
              <input
                type="checkbox"
                checked={override.ingressTls ?? defaults?.ingressTls ?? false}
                onChange={(e) => onUpdateOverride({ ingressTls: e.target.checked })}
              />
              HTTPS
            </label>
            {(override.ingressTls ?? defaults?.ingressTls) && (
              <label className="flex h-9 items-center gap-2 rounded-md border bg-card px-3 text-sm">
                <input
                  type="checkbox"
                  checked={override.ingressTlsRedirect ?? defaults?.ingressTlsRedirect ?? false}
                  onChange={(e) => onUpdateOverride({ ingressTlsRedirect: e.target.checked })}
                />
                HTTP 重定向到 HTTPS
              </label>
            )}
            <label className="flex h-9 items-center gap-2 rounded-md border bg-card px-3 text-sm">
              <input
                type="checkbox"
                checked={override.ingressRewrite ?? defaults?.ingressRewrite ?? false}
                onChange={(e) => onUpdateOverride({ ingressRewrite: e.target.checked, ingressRewritePath: e.target.checked ? (override.ingressRewritePath || '/') : undefined })}
              />
              是否重写
            </label>
            {(override.ingressRewrite ?? defaults?.ingressRewrite) && (
              <CompactField label="重写路径">
                <Input
                  value={override.ingressRewritePath ?? ''}
                  placeholder={defaults?.ingressRewritePath || '/'}
                  onChange={(e) => onUpdateOverride({ ingressRewritePath: e.target.value })}
                />
              </CompactField>
            )}
          </div>
        </ConfigSection>
      )}
    </div>
  );
}

function StageEnvVarsEditor({ envVars, defaultEnvVars, onChange }: {
  envVars: Array<{ name: string; value: string }>;
  defaultEnvVars?: Array<{ name: string; value: string }>;
  onChange: (vars: Array<{ name: string; value: string }>) => void;
}) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">环境变量 ({envVars.length} 项覆盖{defaultEnvVars?.length ? `，默认 ${defaultEnvVars.length} 项` : ''})</span>
        <Button variant="ghost" size="sm" className="h-6 text-xs" onClick={() => onChange([...envVars, { name: '', value: '' }])}>
          <Plus className="mr-1 h-3 w-3" />添加
        </Button>
      </div>
      {envVars.map((item, i) => (
        <div key={i} className="flex gap-2">
          <Input className="flex-1" placeholder="变量名" value={item.name} onChange={(e) => { const next = [...envVars]; next[i] = { ...item, name: e.target.value }; onChange(next); }} />
          <Input className="flex-1" placeholder="变量值" value={item.value} onChange={(e) => { const next = [...envVars]; next[i] = { ...item, value: e.target.value }; onChange(next); }} />
          <Button variant="ghost" size="icon" className="h-9 w-9 shrink-0" onClick={() => onChange(envVars.filter((_, j) => j !== i))}><Trash2 className="h-3 w-3" /></Button>
        </div>
      ))}
    </div>
  );
}

function StageSecretRefsEditor({ secretRefs, defaultSecretRefs, onChange }: {
  secretRefs: Array<{ name: string; secretRef: string }>;
  defaultSecretRefs?: Array<{ name: string; secretRef: string }>;
  onChange: (refs: Array<{ name: string; secretRef: string }>) => void;
}) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">敏感配置 ({secretRefs.length} 项覆盖{defaultSecretRefs?.length ? `，默认 ${defaultSecretRefs.length} 项` : ''})</span>
        <Button variant="ghost" size="sm" className="h-6 text-xs" onClick={() => onChange([...secretRefs, { name: '', secretRef: '' }])}>
          <Plus className="mr-1 h-3 w-3" />添加
        </Button>
      </div>
      {secretRefs.map((item, i) => (
        <div key={i} className="flex gap-2">
          <Input className="flex-1" placeholder="名称" value={item.name} onChange={(e) => { const next = [...secretRefs]; next[i] = { ...item, name: e.target.value }; onChange(next); }} />
          <Input className="flex-1" placeholder="Secret 引用" value={item.secretRef} onChange={(e) => { const next = [...secretRefs]; next[i] = { ...item, secretRef: e.target.value }; onChange(next); }} />
          <Button variant="ghost" size="icon" className="h-9 w-9 shrink-0" onClick={() => onChange(secretRefs.filter((_, j) => j !== i))}><Trash2 className="h-3 w-3" /></Button>
        </div>
      ))}
    </div>
  );
}

function StageConfigFilesEditor({ configFiles, defaultConfigFiles, onChange }: {
  configFiles: Array<{ mountPath: string; content: string }>;
  defaultConfigFiles?: Array<{ mountPath: string; content: string }>;
  onChange: (files: Array<{ mountPath: string; content: string }>) => void;
}) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground">配置文件 ({configFiles.length} 项覆盖{defaultConfigFiles?.length ? `，默认 ${defaultConfigFiles.length} 项` : ''})</span>
        <Button variant="ghost" size="sm" className="h-6 text-xs" onClick={() => onChange([...configFiles, { mountPath: '', content: '' }])}>
          <Plus className="mr-1 h-3 w-3" />添加
        </Button>
      </div>
      {configFiles.map((item, i) => (
        <div key={i} className="space-y-1">
          <div className="flex gap-2">
            <Input className="flex-1" placeholder="挂载路径" value={item.mountPath} onChange={(e) => { const next = [...configFiles]; next[i] = { ...item, mountPath: e.target.value }; onChange(next); }} />
            <Button variant="ghost" size="icon" className="h-9 w-9 shrink-0" onClick={() => onChange(configFiles.filter((_, j) => j !== i))}><Trash2 className="h-3 w-3" /></Button>
          </div>
          <textarea className="w-full rounded-md border bg-card p-2 text-xs font-mono" rows={3} placeholder="文件内容" value={item.content} onChange={(e) => { const next = [...configFiles]; next[i] = { ...item, content: e.target.value }; onChange(next); }} />
        </div>
      ))}
    </div>
  );
}

function normalizeKV(raw?: any[]): Array<{ name: string; value: string }> {
  if (!raw?.length) return [];
  return raw.map((item) => ({ name: item.name || '', value: item.value || '' })).filter((item) => item.name);
}

function normalizeSecretRefsCfg(raw?: any[]): Array<{ name: string; secretRef: string }> {
  if (!raw?.length) return [];
  return raw.map((item) => ({ name: item.name || '', secretRef: item.secret_ref || item.secretRef || '' })).filter((item) => item.name);
}

function normalizeConfigFilesCfg(raw?: any[]): Array<{ mountPath: string; content: string }> {
  if (!raw?.length) return [];
  return raw.map((item) => ({ mountPath: item.mount_path || item.mountPath || '', content: item.content || '' })).filter((item) => item.mountPath);
}

function Metric({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-md border bg-muted/30 p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={cn('mt-1 truncate text-sm font-medium', mono && 'mono')}>{value}</div>
    </div>
  );
}

function policyText(policy: string) {
  if (policy === 'auto') return '自动';
  if (policy === 'approval_required') return '需审批';
  return '手动';
}

function stageRequiresVerification(stage: Stage) {
  return Boolean(stage.requiresVerification);
}
