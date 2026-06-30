import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Eye, EyeOff, Lock, RefreshCw, X } from 'lucide-react';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../components/ui/tabs';
import { API_BASE_URL } from '../api/client';
import { cn } from '../lib/utils';

// ====== API helpers (login-specific, no auth header) ======
async function authRequest<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });
  const text = await res.text();
  let data: any;
  try {
    data = JSON.parse(text);
  } catch {
    data = null;
  }
  if (!res.ok) {
    const msg = data?.error?.message || `请求失败 (HTTP ${res.status})`;
    throw new Error(msg);
  }
  return data as T;
}

// ====== Token storage ======
function saveAuth(token: any, userName?: string, userId?: string) {
  if (typeof token === 'string') {
    localStorage.setItem('paas_token', token);
  } else if (token && token.access_token) {
    localStorage.setItem('paas_token', token.access_token);
    if (token.refresh_token) localStorage.setItem('paas_refresh_token', token.refresh_token);
    if (token.expires_at) localStorage.setItem('paas_token_expires', token.expires_at);
  }
  if (userName) localStorage.setItem('paas_user_name', userName);
  if (userId) localStorage.setItem('paas_actor_id', userId);
}

function isAuthenticated(): boolean {
  const token = localStorage.getItem('paas_token');
  if (!token) return false;
  const expires = localStorage.getItem('paas_token_expires');
  if (expires && new Date(expires) <= new Date()) {
    localStorage.removeItem('paas_token');
    localStorage.removeItem('paas_refresh_token');
    localStorage.removeItem('paas_token_expires');
    return false;
  }
  return true;
}

// ====== Captcha generator ======
function generateCaptcha(canvas: HTMLCanvasElement): string {
  const ctx = canvas.getContext('2d')!;
  const w = canvas.width;
  const h = canvas.height;
  ctx.clearRect(0, 0, w, h);
  ctx.fillStyle = '#f3f4f6';
  ctx.fillRect(0, 0, w, h);

  for (let i = 0; i < 4; i++) {
    ctx.strokeStyle = `hsl(${Math.random() * 360}, 40%, 70%)`;
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(Math.random() * w, Math.random() * h);
    ctx.lineTo(Math.random() * w, Math.random() * h);
    ctx.stroke();
  }

  for (let i = 0; i < 30; i++) {
    ctx.fillStyle = `hsl(${Math.random() * 360}, 40%, 60%)`;
    ctx.fillRect(Math.random() * w, Math.random() * h, 2, 2);
  }

  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789';
  let code = '';
  ctx.font = 'bold 22px monospace';
  ctx.textBaseline = 'middle';
  for (let i = 0; i < 4; i++) {
    const c = chars[Math.floor(Math.random() * chars.length)];
    code += c;
    ctx.save();
    ctx.translate(20 + i * 25, h / 2 + (Math.random() * 10 - 5));
    ctx.rotate((Math.random() - 0.5) * 0.4);
    ctx.fillStyle = `hsl(${Math.random() * 360}, 60%, 35%)`;
    ctx.fillText(c, 0, 0);
    ctx.restore();
  }
  return code;
}

// ====== Toast component ======
function Toast({ message, type, onClose }: { message: string; type: 'error' | 'success'; onClose: () => void }) {
  useEffect(() => {
    const timer = setTimeout(onClose, 4000);
    return () => clearTimeout(timer);
  }, [onClose]);

  return (
    <div
      className={cn(
        'fixed top-6 right-6 z-[200] max-w-[360px] rounded-md border px-5 py-3 text-sm shadow-lg animate-in slide-in-from-right-4',
        type === 'error' ? 'border-red-200 bg-red-50 text-red-800' : 'border-green-200 bg-green-50 text-green-800'
      )}
    >
      {message}
    </div>
  );
}

// ====== Main component ======
export function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  // If already authenticated, redirect
  useEffect(() => {
    if (isAuthenticated()) {
      navigate('/', { replace: true });
    }
  }, [navigate]);

  // Handle OIDC callback
  useEffect(() => {
    const code = searchParams.get('code');
    const state = searchParams.get('state');
    const providerId = searchParams.get('provider') || localStorage.getItem('paas_oidc_provider');
    if (code && providerId) {
      authRequest<any>(`/api/auth/oidc/${providerId}/callback?state=${encodeURIComponent(state || '')}&code=${encodeURIComponent(code)}`)
        .then((data) => {
          saveAuth(data.token, data.user?.display_name || data.user?.username, data.user?.id);
          localStorage.removeItem('paas_oidc_provider');
          navigate('/', { replace: true });
        })
        .catch((err) => {
          setToast({ message: err.message || 'SSO 登录失败', type: 'error' });
        });
    }
  }, [searchParams, navigate]);

  const [toast, setToast] = useState<{ message: string; type: 'error' | 'success' } | null>(null);
  const [showRegister, setShowRegister] = useState(false);
  const [showForgot, setShowForgot] = useState(false);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-6">
      <div className="w-full max-w-[400px]">
        <div className="rounded-lg border bg-card p-8 shadow-sm">
          <div className="mb-6">
            <h1 className="text-xl font-semibold tracking-tight">登录 CloudDeliver</h1>
            <p className="mt-1 text-sm text-muted-foreground">登录您的账号以访问控制台</p>
          </div>

          <Tabs defaultValue="local">
            <TabsList className="mb-5 w-full">
              <TabsTrigger value="local" className="flex-1">账号密码登录</TabsTrigger>
              <TabsTrigger value="oidc" className="flex-1">企业 SSO 登录</TabsTrigger>
            </TabsList>

            <TabsContent value="local">
              <LocalLoginForm
                onSuccess={() => navigate('/', { replace: true })}
                onToast={setToast}
                onRegister={() => setShowRegister(true)}
                onForgot={() => setShowForgot(true)}
              />
            </TabsContent>

            <TabsContent value="oidc">
              <OIDCLoginPanel onToast={setToast} />
            </TabsContent>
          </Tabs>
        </div>

        <p className="mt-6 text-center text-xs text-muted-foreground">
          © 2026 CloudDeliver · 企业内部系统
        </p>
      </div>

      {/* Register dialog */}
      {showRegister && (
        <DialogOverlay onClose={() => setShowRegister(false)}>
          <RegisterForm
            onClose={() => setShowRegister(false)}
            onSuccess={() => navigate('/', { replace: true })}
            onToast={setToast}
          />
        </DialogOverlay>
      )}

      {/* Forgot password dialog */}
      {showForgot && (
        <DialogOverlay onClose={() => setShowForgot(false)}>
          <ForgotPasswordForm
            onClose={() => setShowForgot(false)}
            onToast={setToast}
          />
        </DialogOverlay>
      )}

      {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}
    </div>
  );
}

// ====== Local Login Form ======
function LocalLoginForm({
  onSuccess,
  onToast,
  onRegister,
  onForgot,
}: {
  onSuccess: () => void;
  onToast: (t: { message: string; type: 'error' | 'success' }) => void;
  onRegister: () => void;
  onForgot: () => void;
}) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [captchaInput, setCaptchaInput] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState<{ username?: string; password?: string; captcha?: string }>({});

  const captchaCanvasRef = useRef<HTMLCanvasElement>(null);
  const captchaCodeRef = useRef('');

  const refreshCaptcha = useCallback(() => {
    if (captchaCanvasRef.current) {
      captchaCodeRef.current = generateCaptcha(captchaCanvasRef.current);
    }
  }, []);

  useEffect(() => {
    refreshCaptcha();
  }, [refreshCaptcha]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const newErrors: typeof errors = {};
    if (!username.trim()) newErrors.username = '请输入用户名';
    if (!password) newErrors.password = '请输入密码';
    if (!captchaInput.trim()) newErrors.captcha = '请输入验证码';
    setErrors(newErrors);
    if (Object.keys(newErrors).length > 0) return;

    if (captchaInput.toLowerCase() !== captchaCodeRef.current.toLowerCase()) {
      setErrors({ captcha: '验证码错误' });
      refreshCaptcha();
      setCaptchaInput('');
      return;
    }

    setLoading(true);
    try {
      let data: any;
      try {
        data = await authRequest<any>('/api/auth/local/login', {
          method: 'POST',
          body: JSON.stringify({ account: username.trim(), password }),
        });
        saveAuth(data.token, data.userName, data.userId);
      } catch {
        data = await authRequest<any>('/api/auth/login', {
          method: 'POST',
          body: JSON.stringify({ username: username.trim(), password }),
        });
        saveAuth(data.token, data.user?.display_name || data.user?.username, data.user?.id);
      }
      onToast({ message: '登录成功，正在跳转...', type: 'success' });
      setTimeout(onSuccess, 400);
    } catch (err: any) {
      onToast({ message: err.message || '登录失败，请检查用户名和密码', type: 'error' });
      refreshCaptcha();
      setCaptchaInput('');
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} noValidate>
      <div className="space-y-4">
        <div>
          <label className="mb-1.5 block text-sm font-medium">用户名 / 邮箱</label>
          <Input
            type="text"
            placeholder="请输入用户名或邮箱"
            autoComplete="username"
            value={username}
            onChange={(e) => { setUsername(e.target.value); setErrors((prev) => ({ ...prev, username: undefined })); }}
            className={cn(errors.username && 'border-destructive ring-1 ring-destructive/20')}
            autoFocus
          />
          {errors.username && <p className="mt-1 text-xs text-destructive">{errors.username}</p>}
        </div>

        <div>
          <label className="mb-1.5 block text-sm font-medium">密码</label>
          <div className="relative">
            <Input
              type={showPassword ? 'text' : 'password'}
              placeholder="请输入密码"
              autoComplete="current-password"
              value={password}
              onChange={(e) => { setPassword(e.target.value); setErrors((prev) => ({ ...prev, password: undefined })); }}
              className={cn('pr-10', errors.password && 'border-destructive ring-1 ring-destructive/20')}
            />
            <button
              type="button"
              className="absolute right-0 top-0 flex h-9 w-9 items-center justify-center text-muted-foreground hover:text-foreground"
              onClick={() => setShowPassword(!showPassword)}
              aria-label={showPassword ? '隐藏密码' : '显示密码'}
            >
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
          {errors.password && <p className="mt-1 text-xs text-destructive">{errors.password}</p>}
        </div>

        <div>
          <label className="mb-1.5 block text-sm font-medium">验证码</label>
          <div className="flex gap-2">
            <Input
              type="text"
              placeholder="请输入验证码"
              maxLength={6}
              autoComplete="off"
              value={captchaInput}
              onChange={(e) => { setCaptchaInput(e.target.value); setErrors((prev) => ({ ...prev, captcha: undefined })); }}
              className={cn('flex-1', errors.captcha && 'border-destructive ring-1 ring-destructive/20')}
            />
            <button
              type="button"
              className="group relative h-9 w-[120px] shrink-0 overflow-hidden rounded-md border"
              onClick={() => { refreshCaptcha(); setCaptchaInput(''); }}
              title="点击刷新验证码"
            >
              <canvas ref={captchaCanvasRef} width={120} height={36} className="h-full w-full" />
              <div className="absolute inset-0 flex items-center justify-center bg-black/50 opacity-0 transition-opacity group-hover:opacity-100">
                <RefreshCw className="h-4 w-4 text-white" />
              </div>
            </button>
          </div>
          {errors.captcha && <p className="mt-1 text-xs text-destructive">{errors.captcha}</p>}
        </div>

        <div className="flex items-center justify-between">
          <button type="button" className="text-sm text-primary hover:underline" onClick={onForgot}>
            忘记密码？
          </button>
          <button type="button" className="text-sm text-primary hover:underline" onClick={onRegister}>
            注册账号
          </button>
        </div>

        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? (
            <span className="flex items-center gap-2">
              <span className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
              登录中...
            </span>
          ) : (
            '登录'
          )}
        </Button>
      </div>
    </form>
  );
}

// ====== OIDC Login Panel ======
function OIDCLoginPanel({ onToast }: { onToast: (t: { message: string; type: 'error' | 'success' }) => void }) {
  const [providers, setProviders] = useState<Array<{ id: string; name: string }>>([]);

  useEffect(() => {
    authRequest<any>('/api/auth/oidc/providers')
      .then((data) => {
        const items = (data.items || []).filter((p: any) => p.enabled);
        if (items.length > 0) setProviders(items);
      })
      .catch(() => {});
  }, []);

  const startOIDC = async (providerId?: string) => {
    try {
      let data: any;
      if (providerId) {
        data = await authRequest<any>(`/api/auth/oidc/${providerId}/start`);
      } else {
        data = await authRequest<any>('/api/auth/oidc/start', { method: 'POST' });
      }
      if (data.redirect_url) {
        if (providerId) localStorage.setItem('paas_oidc_provider', providerId);
        window.location.href = data.redirect_url;
      }
    } catch (err: any) {
      onToast({ message: err.message || 'SSO 登录启动失败', type: 'error' });
    }
  };

  return (
    <div className="space-y-4 py-4 text-center">
      <p className="text-sm text-muted-foreground">
        通过企业单点登录（SSO）认证快速访问平台，无需单独的平台账号密码。
      </p>
      <div className="space-y-3">
        {providers.length > 0 ? (
          providers.map((provider) => (
            <Button
              key={provider.id}
              variant="outline"
              className="w-full gap-2"
              onClick={() => startOIDC(provider.id)}
            >
              <Lock className="h-4 w-4" />
              通过 {provider.name} 登录
            </Button>
          ))
        ) : (
          <Button variant="outline" className="w-full gap-2" onClick={() => startOIDC()}>
            <Lock className="h-4 w-4" />
            通过企业 SSO 登录
          </Button>
        )}
      </div>
    </div>
  );
}

// ====== Dialog Overlay ======
function DialogOverlay({ children, onClose }: { children: React.ReactNode; onClose: () => void }) {
  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handleEsc);
    return () => document.removeEventListener('keydown', handleEsc);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      {children}
    </div>
  );
}

// ====== Register Form ======
function RegisterForm({
  onClose,
  onSuccess,
  onToast,
}: {
  onClose: () => void;
  onSuccess: () => void;
  onToast: (t: { message: string; type: 'error' | 'success' }) => void;
}) {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password) {
      onToast({ message: '请填写用户名和密码', type: 'error' });
      return;
    }
    if (password.length < 8) {
      onToast({ message: '密码至少 8 位', type: 'error' });
      return;
    }
    if (password !== confirm) {
      onToast({ message: '两次输入的密码不一致', type: 'error' });
      return;
    }

    setLoading(true);
    try {
      let data: any;
      try {
        data = await authRequest<any>('/api/auth/local/register', {
          method: 'POST',
          body: JSON.stringify({ username: username.trim(), password, displayName: username.trim(), email }),
        });
        saveAuth(data.token, data.userName, data.userId);
      } catch {
        data = await authRequest<any>('/api/auth/register', {
          method: 'POST',
          body: JSON.stringify({ username: username.trim(), password, display_name: username.trim(), email }),
        });
        saveAuth(data.token, data.user?.display_name || data.user?.username, data.user?.id);
      }
      onToast({ message: '注册成功，正在跳转...', type: 'success' });
      setTimeout(onSuccess, 400);
    } catch (err: any) {
      onToast({ message: err.message || '注册失败，请稍后重试', type: 'error' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="w-full max-w-[400px] rounded-lg border bg-card p-8 shadow-lg">
      <div className="mb-1 flex items-center justify-between">
        <h2 className="text-lg font-semibold">注册账号</h2>
        <button className="text-muted-foreground hover:text-foreground" onClick={onClose}>
          <X className="h-5 w-5" />
        </button>
      </div>
      <p className="mb-6 text-sm text-muted-foreground">创建本地平台账号</p>

      <form onSubmit={handleSubmit} noValidate className="space-y-4">
        <div>
          <label className="mb-1.5 block text-sm font-medium">用户名</label>
          <Input placeholder="请输入用户名" value={username} onChange={(e) => setUsername(e.target.value)} />
        </div>
        <div>
          <label className="mb-1.5 block text-sm font-medium">邮箱</label>
          <Input type="email" placeholder="请输入邮箱地址" value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div>
          <label className="mb-1.5 block text-sm font-medium">密码</label>
          <Input type="password" placeholder="至少 8 位，含字母和数字" value={password} onChange={(e) => setPassword(e.target.value)} />
        </div>
        <div>
          <label className="mb-1.5 block text-sm font-medium">确认密码</label>
          <Input type="password" placeholder="请再次输入密码" value={confirm} onChange={(e) => setConfirm(e.target.value)} />
        </div>
        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? '注册中...' : '注册'}
        </Button>
        <p className="text-center text-sm text-muted-foreground">
          已有账号？{' '}
          <button type="button" className="text-primary hover:underline" onClick={onClose}>
            立即登录
          </button>
        </p>
      </form>
    </div>
  );
}

// ====== Forgot Password Form ======
function ForgotPasswordForm({
  onClose,
  onToast,
}: {
  onClose: () => void;
  onToast: (t: { message: string; type: 'error' | 'success' }) => void;
}) {
  const [email, setEmail] = useState('');
  const [captchaInput, setCaptchaInput] = useState('');
  const [loading, setLoading] = useState(false);
  const captchaCanvasRef = useRef<HTMLCanvasElement>(null);
  const captchaCodeRef = useRef('');

  const refreshCaptcha = useCallback(() => {
    if (captchaCanvasRef.current) {
      captchaCodeRef.current = generateCaptcha(captchaCanvasRef.current);
    }
  }, []);

  useEffect(() => {
    refreshCaptcha();
  }, [refreshCaptcha]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email.trim()) {
      onToast({ message: '请输入注册邮箱', type: 'error' });
      return;
    }
    if (!captchaInput.trim()) {
      onToast({ message: '请输入验证码', type: 'error' });
      return;
    }
    if (captchaInput.toLowerCase() !== captchaCodeRef.current.toLowerCase()) {
      onToast({ message: '验证码错误', type: 'error' });
      refreshCaptcha();
      setCaptchaInput('');
      return;
    }

    setLoading(true);
    try {
      await authRequest<any>('/api/auth/reset-password-request', {
        method: 'POST',
        body: JSON.stringify({ email: email.trim() }),
      });
      onToast({ message: '重置链接已发送到您的邮箱', type: 'success' });
      onClose();
    } catch (err: any) {
      onToast({ message: err.message || '发送失败，请稍后重试', type: 'error' });
      refreshCaptcha();
      setCaptchaInput('');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="w-full max-w-[400px] rounded-lg border bg-card p-8 shadow-lg">
      <div className="mb-1 flex items-center justify-between">
        <h2 className="text-lg font-semibold">忘记密码</h2>
        <button className="text-muted-foreground hover:text-foreground" onClick={onClose}>
          <X className="h-5 w-5" />
        </button>
      </div>
      <p className="mb-6 text-sm text-muted-foreground">输入注册邮箱，我们将发送重置密码链接</p>

      <form onSubmit={handleSubmit} noValidate className="space-y-4">
        <div>
          <label className="mb-1.5 block text-sm font-medium">邮箱</label>
          <Input type="email" placeholder="请输入注册邮箱" value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div>
          <label className="mb-1.5 block text-sm font-medium">验证码</label>
          <div className="flex gap-2">
            <Input
              type="text"
              placeholder="请输入验证码"
              maxLength={6}
              autoComplete="off"
              value={captchaInput}
              onChange={(e) => setCaptchaInput(e.target.value)}
              className="flex-1"
            />
            <button
              type="button"
              className="group relative h-9 w-[120px] shrink-0 overflow-hidden rounded-md border"
              onClick={() => { refreshCaptcha(); setCaptchaInput(''); }}
              title="点击刷新验证码"
            >
              <canvas ref={captchaCanvasRef} width={120} height={36} className="h-full w-full" />
              <div className="absolute inset-0 flex items-center justify-center bg-black/50 opacity-0 transition-opacity group-hover:opacity-100">
                <RefreshCw className="h-4 w-4 text-white" />
              </div>
            </button>
          </div>
        </div>
        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? '发送中...' : '发送重置链接'}
        </Button>
        <p className="text-center text-sm text-muted-foreground">
          想起密码了？{' '}
          <button type="button" className="text-primary hover:underline" onClick={onClose}>
            返回登录
          </button>
        </p>
      </form>
    </div>
  );
}
