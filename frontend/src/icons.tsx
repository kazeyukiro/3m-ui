/**
 * Panel icons — Lucide under the hood.
 * Prefer semantic exports (IconNav*, IconAction*) so the same glyph is not reused
 * across unrelated UI slots. Unused action/legacy aliases are omitted; add new Icon* only when a page needs them.
 */
import React from 'react';
import type { LucideProps } from 'lucide-react';
import {
  LayoutDashboard,
  Fingerprint,
  ShieldCheck,
  BookOpen,
  Terminal,
  Boxes,
  Sparkles,
  Route,
  Radio,
  Users,
  Share2,
  Activity,
  Network,
  Cpu,
  SlidersHorizontal,
  Settings,
  LogOut,
  CirclePlus,
  ListPlus,
  Rows3,
  UserPlus,
  Server,
  Plus,
  Trash2,
  SquarePen,
  PenLine,
  RefreshCw,
  RotateCw,
  RotateCcw,
  Copy,
  ClipboardCopy,
  Link2,
  ExternalLink,
  QrCode,
  Save,
  BadgeCheck,
  LayoutGrid,
  AppWindow,
  Wrench,
  FolderKanban,
  ArrowLeft,
  ChevronUp,
  ChevronDown,
  HeartPulse,
  Monitor,
  HardDrive,
  LogIn,
  LineChart,
  Check,
  CheckCheck,
  GitBranch,
  History,
  Power,
  FileDiff,
  Info,
  CircleHelp,
  CloudDownload,
  CloudUpload,
  PackagePlus,
  Cable,
  Gauge,
  PanelLeftClose,
  PanelLeftOpen,
  CircleMinus,
  Eraser,
  PlayCircle,
  CirclePlay,
  Square,
  Download,
  Send,
  MessageCircle,
  MoreHorizontal,
  FileText as LucideFileText,
  FileKey2,
  Shield,
  User,
  Lock,
  Globe,
  Palette,
  KeyRound,
  ListRestart,
  Ban,
  CircleX,
} from 'lucide-react';

type IconProps = LucideProps & {
  style?: React.CSSProperties & { fontSize?: number | string };
  className?: string;
  twoToneColor?: string;
  spin?: boolean;
  rotate?: number;
};

function wrap(Icon: React.ComponentType<LucideProps>, displayName: string) {
  const Comp = React.forwardRef<SVGSVGElement, IconProps>(function LucideIcon(
    { style, className, spin, rotate, size, ...rest },
    ref,
  ) {
    const fontSize = style?.fontSize;
    const resolvedSize =
      size ??
      (typeof fontSize === 'number'
        ? fontSize
        : typeof fontSize === 'string' && fontSize.endsWith('px')
          ? parseFloat(fontSize)
          : 16);
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

/* —— Navigation (sidebar / mobile) — unique glyphs —— */
export const IconNavDashboard = wrap(LayoutDashboard, 'IconNavDashboard');
export const IconNavListeners = wrap(Radio, 'IconNavListeners');
export const IconNavUsers = wrap(Users, 'IconNavUsers');
export const IconNavShare = wrap(Share2, 'IconNavShare');
export const IconNavTraffic = wrap(Activity, 'IconNavTraffic');
export const IconNavCluster = wrap(Boxes, 'IconNavCluster');
export const IconNavRouting = wrap(Route, 'IconNavRouting');
export const IconNavCore = wrap(Cpu, 'IconNavCore');
export const IconNavLogs = wrap(Terminal, 'IconNavLogs');
export const IconNavConfig = wrap(SlidersHorizontal, 'IconNavConfig');
export const IconNavSettings = wrap(Settings, 'IconNavSettings');
export const IconNavLogout = wrap(LogOut, 'IconNavLogout');

/* —— Actions — prefer these over generic Plus/Reload —— */
export const IconAddNode = wrap(CirclePlus, 'IconAddNode');
export const IconQuickCreate = wrap(Sparkles, 'IconQuickCreate');
export const IconAddUser = wrap(UserPlus, 'IconAddUser');
export const IconAddRemote = wrap(Server, 'IconAddRemote');
export const IconAddGeneric = wrap(Plus, 'IconAddGeneric');
export const IconAddField = wrap(ListPlus, 'IconAddField');
export const IconRemoveField = wrap(CircleMinus, 'IconRemoveField');
export const IconDelete = wrap(Trash2, 'IconDelete');
export const IconDeleteAlt = wrap(CircleX, 'IconDeleteAlt');
export const IconEdit = wrap(SquarePen, 'IconEdit');
export const IconEditAlt = wrap(PenLine, 'IconEditAlt');
export const IconRefreshList = wrap(RefreshCw, 'IconRefreshList');
export const IconReloadCore = wrap(RotateCw, 'IconReloadCore');
export const IconRotateToken = wrap(KeyRound, 'IconRotateToken');
export const IconRestart = wrap(RotateCcw, 'IconRestart');
export const IconCopy = wrap(Copy, 'IconCopy');
export const IconCopyAlt = wrap(ClipboardCopy, 'IconCopyAlt');
export const IconLink = wrap(Link2, 'IconLink');
export const IconExternal = wrap(ExternalLink, 'IconExternal');
export const IconQr = wrap(QrCode, 'IconQr');
export const IconSave = wrap(Save, 'IconSave');
export const IconCheck = wrap(Check, 'IconCheck');
export const IconPlay = wrap(PlayCircle, 'IconPlay');
export const IconStop = wrap(Square, 'IconStop');
export const IconDownload = wrap(Download, 'IconDownload');
export const IconSend = wrap(Send, 'IconSend');
export const IconMore = wrap(MoreHorizontal, 'IconMore');
export const IconClear = wrap(Eraser, 'IconClear');
export const IconBan = wrap(Ban, 'IconBan');
export const IconHealth = wrap(HeartPulse, 'IconHealth');
export const IconSync = wrap(ListRestart, 'IconSync');
export const IconLogin = wrap(LogIn, 'IconLogin');
export const IconDesktop = wrap(Monitor, 'IconDesktop');
export const IconDisk = wrap(HardDrive, 'IconDisk');
export const IconCert = wrap(BadgeCheck, 'IconCert');
export const IconShield = wrap(Shield, 'IconShield');
export const IconUser = wrap(User, 'IconUser');
export const IconLock = wrap(Lock, 'IconLock');
export const IconGlobe = wrap(Globe, 'IconGlobe');
export const IconTheme = wrap(Palette, 'IconTheme');
export const IconMenuOpen = wrap(PanelLeftOpen, 'IconMenuOpen');
export const IconMenuClose = wrap(PanelLeftClose, 'IconMenuClose');
export const IconBack = wrap(ArrowLeft, 'IconBack');
export const IconPower = wrap(Power, 'IconPower');
export const IconHistory = wrap(History, 'IconHistory');
export const IconBranch = wrap(GitBranch, 'IconBranch');
export const IconDiff = wrap(FileDiff, 'IconDiff');
export const IconInfo = wrap(Info, 'IconInfo');
export const IconCloudDown = wrap(CloudDownload, 'IconCloudDown');
export const IconCloudUp = wrap(CloudUpload, 'IconCloudUp');
export const IconApi = wrap(Cable, 'IconApi');
export const IconFile = wrap(LucideFileText, 'IconFile');
export const IconChart = wrap(LineChart, 'IconChart');
export const IconGrid = wrap(LayoutGrid, 'IconGrid');


/* —— Routing page —— unique vs generic add/play/check —— */
export const IconAddRule = wrap(Rows3, 'IconAddRule');
export const IconAddGroup = wrap(FolderKanban, 'IconAddGroup');
export const IconMoveUp = wrap(ChevronUp, 'IconMoveUp');
export const IconMoveDown = wrap(ChevronDown, 'IconMoveDown');
export const IconSaveRules = wrap(CheckCheck, 'IconSaveRules');
export const IconApplyRules = wrap(CirclePlay, 'IconApplyRules');

export const IconSettingsPanel = wrap(AppWindow, 'IconSettingsPanel');
export const IconSettingsAccess = wrap(Fingerprint, 'IconSettingsAccess');
export const IconSettingsTelegram = wrap(MessageCircle, 'IconSettingsTelegram');
export const IconSettingsSecurity = wrap(ShieldCheck, 'IconSettingsSecurity');
export const IconSettingsSubPage = wrap(BookOpen, 'IconSettingsSubPage');
export const IconSettingsSSL = wrap(FileKey2, 'IconSettingsSSL');
export const IconSettingsProxy = wrap(Network, 'IconSettingsProxy');
export const IconSettingsTraffic = wrap(Gauge, 'IconSettingsTraffic');
export const IconSettingsAbout = wrap(CircleHelp, 'IconSettingsAbout');
export const IconSettingsOps = wrap(Wrench, 'IconSettingsOps');
export const IconUpdate = wrap(PackagePlus, 'IconUpdate');
