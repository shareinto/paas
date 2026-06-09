import { Result, Spin } from 'antd';
import { useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useSession } from '../app/store';

export function OIDCCallbackPage() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const setSession = useSession((state) => state.setSession);
  useEffect(() => {
    const timer = window.setTimeout(() => {
      if (params.get('code')) {
        setSession('mock-oidc-token', '企业用户');
        navigate('/apps');
      }
    }, 300);
    return () => window.clearTimeout(timer);
  }, [navigate, params, setSession]);
  if (!params.get('code')) {
    return <Result status="error" title="企业身份登录失败" subTitle="回调参数无效，请重新登录" />;
  }
  return <div className="center-page"><Spin tip="正在完成企业身份登录" /></div>;
}
