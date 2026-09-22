/**
 * Panel icons — Lucide under the hood, Ant Design–style *Outlined names for drop-in replacement.
 * Import from `../icons` or `../../icons` instead of `@ant-design/icons`.
 */
import React from 'react';
import type { LucideProps } from 'lucide-react';
import {
  Plus,
  RefreshCw,
  Trash2,
  Pencil,
  Globe,
  Palette,
  Users,
  Link2,
  Share2,
  Copy,
  LayoutDashboard,
  FileText,
  Server,
  User,
  Lock,
  Shield,
  PlayCircle,
  Square,
  RotateCcw,
  Download,
  Send,
  Eraser,
  MoreHorizontal,
  QrCode,
  Save,
  BadgeCheck,
  LayoutGrid,
  Boxes,
  MonitorPlay,
  GitFork,
  Rocket,
  SlidersHorizontal,
  Wrench,
  LogOut,
  ArrowLeft,
  Undo2,
  HeartPulse,
  RefreshCcw,
  Monitor,
  HardDrive,
  LogIn,
  LineChart,
  Check,
  GitBranch,
  History,
  Power,
  FileDiff,
  Info,
  CloudDownload,
  CloudUpload,
  Cable,
  Settings,
  Bell,
  Network,
  Timer,
  PanelLeftClose,
  PanelLeftOpen,
  MinusCircle,
} from 'lucide-react';

type IconProps = LucideProps & {
  /** Ant Design icons often pass style.fontSize — map to Lucide `size`. */
  style?: React.CSSProperties & { fontSize?: number | string };
  className?: string;
  twoToneColor?: string;
  spin?: boolean;
  rotate?: number;
};

function wrap(Icon: React.ComponentType<LucideProps>, displayName: string) {
  const Comp = React.forwardRef<SVGSVGElement, IconProps>(function LucideAntIcon(
    { style, className, spin, rotate, size, ...rest },
    ref,
  ) {
    const fontSize = style?.fontSize;
    const resolvedSize =
      size ??
      (typeof fontSize === 'number' ? fontSize : typeof fontSize === 'string' && fontSize.endsWith('px') ? parseFloat(fontSize) : 16);
    const mergedStyle: React.CSSProperties = {
      ...style,
      ...(spin ? { animation: 'spin 1s linear infinite' } : null),
      ...(rotate != null ? { transform: `rotate(${rotate}deg)` } : null),
      verticalAlign: style?.verticalAlign ?? 'middle',
    };
    return (
      <Icon
        ref={ref}
        size={resolvedSize}
        strokeWidth={1.75}
        className={className}
        style={mergedStyle}
        aria-hidden
        {...rest}
      />
    );
  });
  Comp.displayName = displayName;
  return Comp;
}

export const PlusOutlined = wrap(Plus, 'PlusOutlined');
export const ReloadOutlined = wrap(RefreshCw, 'ReloadOutlined');
export const DeleteOutlined = wrap(Trash2, 'DeleteOutlined');
export const EditOutlined = wrap(Pencil, 'EditOutlined');
export const GlobalOutlined = wrap(Globe, 'GlobalOutlined');
export const BgColorsOutlined = wrap(Palette, 'BgColorsOutlined');
export const TeamOutlined = wrap(Users, 'TeamOutlined');
export const LinkOutlined = wrap(Link2, 'LinkOutlined');
export const ShareAltOutlined = wrap(Share2, 'ShareAltOutlined');
export const CopyOutlined = wrap(Copy, 'CopyOutlined');
export const DashboardOutlined = wrap(LayoutDashboard, 'DashboardOutlined');
export const ProfileOutlined = wrap(FileText, 'ProfileOutlined');
export const CloudServerOutlined = wrap(Server, 'CloudServerOutlined');
export const UserOutlined = wrap(User, 'UserOutlined');
export const LockOutlined = wrap(Lock, 'LockOutlined');
export const SafetyOutlined = wrap(Shield, 'SafetyOutlined');
export const PlayCircleOutlined = wrap(PlayCircle, 'PlayCircleOutlined');
export const StopOutlined = wrap(Square, 'StopOutlined');
export const RedoOutlined = wrap(RotateCcw, 'RedoOutlined');
export const DownloadOutlined = wrap(Download, 'DownloadOutlined');
export const SendOutlined = wrap(Send, 'SendOutlined');
export const ClearOutlined = wrap(Eraser, 'ClearOutlined');
export const MoreOutlined = wrap(MoreHorizontal, 'MoreOutlined');
export const QrcodeOutlined = wrap(QrCode, 'QrcodeOutlined');
export const FileTextOutlined = wrap(FileText, 'FileTextOutlined');
export const SaveOutlined = wrap(Save, 'SaveOutlined');
export const SafetyCertificateOutlined = wrap(BadgeCheck, 'SafetyCertificateOutlined');
export const AppstoreOutlined = wrap(LayoutGrid, 'AppstoreOutlined');
export const DeploymentUnitOutlined = wrap(Boxes, 'DeploymentUnitOutlined');
export const FundProjectionScreenOutlined = wrap(MonitorPlay, 'FundProjectionScreenOutlined');
export const ForkOutlined = wrap(GitFork, 'ForkOutlined');
export const RocketOutlined = wrap(Rocket, 'RocketOutlined');
export const ControlOutlined = wrap(SlidersHorizontal, 'ControlOutlined');
export const ToolOutlined = wrap(Wrench, 'ToolOutlined');
export const LogoutOutlined = wrap(LogOut, 'LogoutOutlined');
export const ArrowLeftOutlined = wrap(ArrowLeft, 'ArrowLeftOutlined');
export const RollbackOutlined = wrap(Undo2, 'RollbackOutlined');
export const MedicineBoxOutlined = wrap(HeartPulse, 'MedicineBoxOutlined');
export const CloudSyncOutlined = wrap(RefreshCcw, 'CloudSyncOutlined');
export const DesktopOutlined = wrap(Monitor, 'DesktopOutlined');
export const HddOutlined = wrap(HardDrive, 'HddOutlined');
export const LoginOutlined = wrap(LogIn, 'LoginOutlined');
export const FundOutlined = wrap(LineChart, 'FundOutlined');
export const CheckOutlined = wrap(Check, 'CheckOutlined');
export const BranchesOutlined = wrap(GitBranch, 'BranchesOutlined');
export const HistoryOutlined = wrap(History, 'HistoryOutlined');
export const PoweroffOutlined = wrap(Power, 'PoweroffOutlined');
export const DiffOutlined = wrap(FileDiff, 'DiffOutlined');
export const InfoCircleOutlined = wrap(Info, 'InfoCircleOutlined');
export const CloudDownloadOutlined = wrap(CloudDownload, 'CloudDownloadOutlined');
export const CloudUploadOutlined = wrap(CloudUpload, 'CloudUploadOutlined');
export const ApiOutlined = wrap(Cable, 'ApiOutlined');
export const SettingOutlined = wrap(Settings, 'SettingOutlined');
export const BellOutlined = wrap(Bell, 'BellOutlined');
export const ClusterOutlined = wrap(Network, 'ClusterOutlined');
export const FieldTimeOutlined = wrap(Timer, 'FieldTimeOutlined');
export const MenuFoldOutlined = wrap(PanelLeftClose, 'MenuFoldOutlined');
export const MenuUnfoldOutlined = wrap(PanelLeftOpen, 'MenuUnfoldOutlined');
export const MinusCircleOutlined = wrap(MinusCircle, 'MinusCircleOutlined');
