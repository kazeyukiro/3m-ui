import type { ThemeConfig } from 'antd';
import { theme } from 'antd';

/**
 * Ant Design tokens tuned toward a Material-like feel (MUI-inspired),
 * without switching component libraries.
 *
 * Goals: indigo primary, softer radii, paper elevation, denser type scale.
 */
export function materialAntTheme(isDark: boolean): ThemeConfig {
  const shared = {
    // MUI default-ish indigo
    colorPrimary: '#6750A4',
    colorInfo: '#6750A4',
    colorSuccess: '#4CAF50',
    colorWarning: '#FB8C00',
    colorError: '#E53935',
    borderRadius: 12,
    borderRadiusLG: 16,
    borderRadiusSM: 8,
    fontFamily:
      '"Roboto", "Helvetica Neue", "Segoe UI", system-ui, -apple-system, "Noto Sans", "PingFang SC", "Microsoft YaHei", sans-serif',
    fontSize: 14,
    controlHeight: 40,
    controlHeightLG: 48,
    controlHeightSM: 32,
    wireframe: false,
    motionDurationMid: '0.2s',
    motionDurationSlow: '0.28s',
  } as const;

  if (isDark) {
    return {
      algorithm: theme.darkAlgorithm,
      token: {
        ...shared,
        colorBgBase: '#121212',
        colorBgContainer: '#1E1E1E',
        colorBgElevated: '#2C2C2C',
        colorBgLayout: '#0E0E0E',
        colorBorder: 'rgba(255, 255, 255, 0.12)',
        colorBorderSecondary: 'rgba(255, 255, 255, 0.08)',
        // WCAG-friendlier secondary text on dark paper
        colorTextSecondary: 'rgba(255, 255, 255, 0.85)',
        colorTextTertiary: 'rgba(255, 255, 255, 0.65)',
        colorTextQuaternary: 'rgba(255, 255, 255, 0.55)',
        boxShadow:
          '0 2px 4px -1px rgba(0,0,0,0.2), 0 4px 5px 0 rgba(0,0,0,0.14), 0 1px 10px 0 rgba(0,0,0,0.12)',
        boxShadowSecondary:
          '0 3px 5px -1px rgba(0,0,0,0.2), 0 6px 10px 0 rgba(0,0,0,0.14), 0 1px 18px 0 rgba(0,0,0,0.12)',
      },
      components: {
        Button: {
          primaryShadow: '0 1px 3px rgba(0,0,0,0.35)',
          defaultShadow: 'none',
          borderRadius: 20,
          controlHeight: 40,
          fontWeight: 500,
        },
        Card: {
          borderRadiusLG: 16,
          paddingLG: 20,
        },
        Menu: {
          itemBorderRadius: 12,
          itemMarginInline: 8,
          itemHeight: 44,
          iconSize: 18,
        },
        Layout: {
          headerBg: '#1E1E1E',
          siderBg: '#1A1A1A',
          bodyBg: '#0E0E0E',
        },
        Input: {
          borderRadius: 12,
          controlHeight: 40,
        },
        Select: {
          borderRadius: 12,
          controlHeight: 40,
        },
        Table: {
          borderRadius: 12,
          headerBorderRadius: 12,
        },
        Modal: {
          borderRadiusLG: 16,
        },
        Tag: {
          borderRadiusSM: 8,
        },
      },
    };
  }

  return {
    algorithm: theme.defaultAlgorithm,
    token: {
      ...shared,
      colorBgBase: '#FFFBFE',
      colorBgContainer: '#FFFFFF',
      colorBgElevated: '#FFFFFF',
      colorBgLayout: '#F3EDF7',
      colorBorder: 'rgba(121, 116, 126, 0.28)',
      colorBorderSecondary: 'rgba(121, 116, 126, 0.16)',
      boxShadow:
        '0 1px 3px rgba(0,0,0,0.08), 0 1px 2px rgba(0,0,0,0.06)',
      boxShadowSecondary:
        '0 3px 6px -2px rgba(0,0,0,0.1), 0 6px 12px rgba(0,0,0,0.06)',
    },
    components: {
      Button: {
        primaryShadow: '0 1px 2px rgba(103, 80, 164, 0.28)',
        defaultShadow: 'none',
        borderRadius: 20,
        controlHeight: 40,
        fontWeight: 500,
      },
      Card: {
        borderRadiusLG: 16,
        paddingLG: 20,
      },
      Menu: {
        itemBorderRadius: 12,
        itemMarginInline: 8,
        itemHeight: 44,
        iconSize: 18,
      },
      Layout: {
        headerBg: '#FFFBFE',
        siderBg: '#FFFBFE',
        bodyBg: '#F3EDF7',
      },
      Input: {
        borderRadius: 12,
        controlHeight: 40,
      },
      Select: {
        borderRadius: 12,
        controlHeight: 40,
      },
      Table: {
        borderRadius: 12,
        headerBorderRadius: 12,
      },
      Modal: {
        borderRadiusLG: 16,
      },
      Tag: {
        borderRadiusSM: 8,
      },
    },
  };
}
