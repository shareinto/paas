import { create } from 'zustand';

type Session = {
  token: string;
  userName: string;
  tenantName: string;
  projectName: string;
  setSession: (token: string, userName: string) => void;
  clear: () => void;
};

export const useSession = create<Session>((set) => ({
  token: window.localStorage.getItem('paas_token') || '',
  userName: window.localStorage.getItem('paas_user') || '平台用户',
  tenantName: '研发中心',
  projectName: '订单平台',
  setSession: (token, userName) => {
    window.localStorage.setItem('paas_token', token);
    window.localStorage.setItem('paas_user', userName);
    set({ token, userName });
  },
  clear: () => {
    window.localStorage.removeItem('paas_token');
    window.localStorage.removeItem('paas_user');
    set({ token: '', userName: '平台用户' });
  }
}));
