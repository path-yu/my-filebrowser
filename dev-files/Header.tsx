import { useState, useRef, useEffect } from 'react';
import {
  Plus,
  Upload,
  Bell,
  User,
  FileText,
  ChevronDown,
  LogOut,
  Settings,
  Users,
  Shield,
  LayoutDashboard,
  Moon,
  Sun,
  Share2,
  Menu,
  X,
} from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { useTheme } from '../context/ThemeContext';
import { useNavigate, useLocation } from 'react-router-dom';
import { PermissionGuard } from './PermissionGuard';

interface HeaderProps {
  onSearch: (keyword: string) => void;
  onCreate?: () => void;
  onBatchUpload?: () => void;
}

export function Header({ onCreate, onBatchUpload }: HeaderProps) {
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const { user, logout } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const navigate = useNavigate();
  const location = useLocation();
  const menuRef = useRef<HTMLDivElement>(null);
  const mobileMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setUserMenuOpen(false);
      }
      if (mobileMenuRef.current && !mobileMenuRef.current.contains(e.target as Node)) {
        setMobileMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const navItems = [
    { label: '图纸管理', path: '/', icon: LayoutDashboard },
    { label: '用户管理', path: '/users', icon: Users, permission: 'user:manage' },
    { label: '角色权限', path: '/roles', icon: Shield, permission: 'role:manage' },
    { label: '分享管理', path: '/shares', icon: Share2, permission: 'share:manage' },
  ];

  const isDashboard = location.pathname === '/' || location.pathname.startsWith('/share/internal');
  const isUserPage = location.pathname === '/users';
  const isRolePage = location.pathname === '/roles';

  const getActiveMenuPath = () => {
    if (location.pathname.startsWith('/share/internal')) {
      return '/';
    }
    return location.pathname;
  };
  const activeMenuPath = getActiveMenuPath();

  return (
    <header className="bg-white border-b border-slate-200 px-2 py-1.5 sm:px-4 sm:py-2 sticky top-0 z-40 dark:bg-slate-800 dark:border-slate-700">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 sm:gap-4">
          <div className="flex items-center gap-2 cursor-pointer" onClick={() => navigate('/')}>
            <div className="w-8 h-8 bg-primary-600 rounded-md flex items-center justify-center flex-shrink-0">
              <FileText className="w-5 h-5 text-white" />
            </div>
            <div className="hidden sm:block">
              <h1 className="text-base font-bold text-slate-800 dark:text-slate-100">压力容器图纸管理平台</h1>
            </div>
          </div>

          <nav className="hidden md:flex items-center gap-0.5 ml-2">
            {navItems.map((item) => {
              const Icon = item.icon;
              const isActive = activeMenuPath === item.path;
              const content = (
                <button
                  key={item.path}
                  onClick={() => navigate(item.path)}
                  className={`flex items-center gap-1 px-2.5 py-1.5 rounded-md text-xs font-medium transition-colors ${
                    isActive
                      ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                      : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-700 dark:hover:text-slate-100'
                  }`}
                >
                  <Icon className="w-3.5 h-3.5 flex-shrink-0" />
                  <span className="hidden sm:inline">{item.label}</span>
                </button>
              );
              if (item.permission) {
                return (
                  <PermissionGuard key={item.path} permission={item.permission}>
                    {content}
                  </PermissionGuard>
                );
              }
              return content;
            })}
          </nav>
        </div>

        <div className="flex items-center gap-1 sm:gap-2">
          <div className="relative md:hidden" ref={mobileMenuRef}>
            <button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="p-1.5 text-slate-600 hover:bg-slate-100 rounded-md transition-colors dark:text-slate-300 dark:hover:bg-slate-700"
            >
              {mobileMenuOpen ? <X className="w-4 h-4" /> : <Menu className="w-4 h-4" />}
            </button>

            {mobileMenuOpen && (
              <div className="absolute left-0 top-full mt-2 w-56 bg-white rounded-xl shadow-xl border border-slate-200 py-1.5 z-50 dark:bg-slate-800 dark:border-slate-700 md:hidden">
                <nav className="flex flex-col gap-0.5 p-1">
                  {navItems.map((item) => {
                    const Icon = item.icon;
                    const isActive = activeMenuPath === item.path;
                    const content = (
                      <button
                        key={item.path}
                        onClick={() => {
                          navigate(item.path);
                          setMobileMenuOpen(false);
                        }}
                        className={`flex items-center gap-2 px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                          isActive
                            ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-700 dark:hover:text-slate-100'
                        }`}
                      >
                        <Icon className="w-4 h-4 flex-shrink-0" />
                        <span className="hidden sm:inline">{item.label}</span>
                      </button>
                    );
                    if (item.permission) {
                      return (
                        <PermissionGuard key={item.path} permission={item.permission}>
                          {content}
                        </PermissionGuard>
                      );
                    }
                    return content;
                  })}
                </nav>
              </div>
            )}
          </div>

          {isDashboard && (
            <>
              <PermissionGuard permission="drawing:create">
                <button
                  onClick={onCreate}
                  className="flex items-center gap-1 px-2 sm:px-3 py-1.5 bg-primary-600 text-white rounded-md hover:bg-primary-700 transition-colors text-xs font-medium"
                >
                  <Plus className="w-3.5 h-3.5 flex-shrink-0" />
                  <span className="hidden sm:inline">新建图纸</span>
                </button>
              </PermissionGuard>

              <PermissionGuard permission="drawing:create">
                <button
                  onClick={onBatchUpload}
                  className="flex items-center gap-1 px-2 sm:px-3 py-1.5 bg-slate-100 text-slate-700 rounded-md hover:bg-slate-200 transition-colors text-xs font-medium dark:bg-slate-700 dark:text-slate-200 dark:hover:bg-slate-600"
                >
                  <Upload className="w-3.5 h-3.5 flex-shrink-0" />
                  <span className="hidden sm:inline">批量上传</span>
                </button>
              </PermissionGuard>
            </>
          )}

          {isUserPage && (
            <PermissionGuard permission="user:create">
              <button
                onClick={onCreate}
                className="flex items-center gap-1 px-2 sm:px-3 py-1.5 bg-primary-600 text-white rounded-md hover:bg-primary-700 transition-colors text-xs font-medium"
              >
                <Plus className="w-3.5 h-3.5 flex-shrink-0" />
                <span className="hidden sm:inline">新建用户</span>
              </button>
            </PermissionGuard>
          )}

          {isRolePage && (
            <PermissionGuard permission="role:create">
              <button
                onClick={onCreate}
                className="flex items-center gap-1 px-2 sm:px-3 py-1.5 bg-primary-600 text-white rounded-md hover:bg-primary-700 transition-colors text-xs font-medium"
              >
                <Plus className="w-3.5 h-3.5 flex-shrink-0" />
                <span className="hidden sm:inline">新建角色</span>
              </button>
            </PermissionGuard>
          )}

          <button className="relative p-1.5 text-slate-600 hover:bg-slate-100 rounded-md transition-colors dark:text-slate-300 dark:hover:bg-slate-700">
            <Bell className="w-4 h-4" />
            <span className="absolute top-1 right-1 w-1.5 h-1.5 bg-red-500 rounded-full"></span>
          </button>

          <button
            onClick={toggleTheme}
            className="p-1.5 text-slate-600 hover:bg-slate-100 rounded-md transition-colors dark:text-slate-300 dark:hover:bg-slate-700"
            title={theme === 'dark' ? '切换到亮色模式' : '切换到暗色模式'}
          >
            {theme === 'dark' ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
          </button>

          <div className="relative pl-2 border-l border-slate-200 dark:border-slate-700" ref={menuRef}>
            <button
              onClick={() => setUserMenuOpen(!userMenuOpen)}
              className="flex items-center gap-2 hover:bg-slate-50 p-1 -m-1 rounded-md transition-colors dark:hover:bg-slate-700"
            >
              <div className="w-7 h-7 bg-primary-100 rounded-full flex items-center justify-center flex-shrink-0 dark:bg-primary-900/30">
                <User className="w-3.5 h-3.5 text-primary-600 dark:text-primary-400" />
              </div>
              <div className="hidden sm:block text-left">
                <p className="font-medium text-slate-800 text-xs dark:text-slate-100">
                  {user?.real_name || user?.username || '用户'}
                </p>
              </div>
              <ChevronDown className={`hidden sm:block w-3.5 h-3.5 text-slate-400 transition-transform ${userMenuOpen ? 'rotate-180' : ''} dark:text-slate-500`} />
            </button>

            {userMenuOpen && (
              <div className="absolute right-[-8px] top-full mt-2 w-screen max-w-[280px] sm:right-0 sm:w-56 bg-white rounded-xl shadow-xl border border-slate-200 py-1.5 z-50 dark:bg-slate-800 dark:border-slate-700">
                <div className="px-4 py-3 border-b border-slate-100 dark:border-slate-700">
                  <p className="font-semibold text-slate-800 text-sm dark:text-slate-100">
                    {user?.real_name || user?.username}
                  </p>
                  <p className="text-xs text-slate-500 dark:text-slate-400">{user?.email || user?.username}</p>
                </div>

                <div className="py-1">
                  <button className="w-full flex items-center gap-2.5 px-4 py-2 text-sm text-slate-700 hover:bg-slate-50 transition-colors dark:text-slate-300 dark:hover:bg-slate-700">
                    <User className="w-4 h-4 text-slate-400" />
                    个人资料
                  </button>
                  <button className="w-full flex items-center gap-2.5 px-4 py-2 text-sm text-slate-700 hover:bg-slate-50 transition-colors dark:text-slate-300 dark:hover:bg-slate-700">
                    <Settings className="w-4 h-4 text-slate-400" />
                    账号设置
                  </button>
                </div>

                <div className="border-t border-slate-100 py-1 dark:border-slate-700">
                  <button
                    onClick={handleLogout}
                    className="w-full flex items-center gap-2.5 px-4 py-2 text-sm text-red-600 hover:bg-red-50 transition-colors dark:hover:bg-red-900/20"
                  >
                    <LogOut className="w-4 h-4" />
                    退出登录我问问
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </header>
  );
}
