import type { ThemeConfig } from 'antd';

/**
 * Material-inspired Ant Design theme (visual only).
 * Does NOT change brand colors — keeps antd default / darkAlgorithm palette.
 * Aims for MUI-like radius, elevation, button weight, and control density.
 */
export function materialThemeConfig(isDark: boolean): ThemeConfig {
  return {
    // Shape & type — Material-ish without touching colorPrimary
    token: {
      borderRadius: 8,
      borderRadiusLG: 12,
      borderRadiusSM: 6,
      borderRadiusXS: 4,
      controlHeight: 36,
      controlHeightLG: 42,
      controlHeightSM: 28,
      fontFamily:
        'Roboto, "Helvetica Neue", "Segoe UI", system-ui, -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", sans-serif',
      fontSize: 14,
      fontWeightStrong: 600,
      lineWidth: 1,
      // Soft elevation (MUI paper-ish); colors left to algorithm
      boxShadow:
        isDark
          ? '0 2px 4px -1px rgba(0,0,0,0.4), 0 4px 5px 0 rgba(0,0,0,0.28), 0 1px 10px 0 rgba(0,0,0,0.24)'
          : '0 2px 4px -1px rgba(0,0,0,0.12), 0 4px 5px 0 rgba(0,0,0,0.08), 0 1px 10px 0 rgba(0,0,0,0.06)',
      boxShadowSecondary:
        isDark
          ? '0 3px 5px -1px rgba(0,0,0,0.45), 0 6px 10px 0 rgba(0,0,0,0.3), 0 1px 18px 0 rgba(0,0,0,0.25)'
          : '0 3px 5px -1px rgba(0,0,0,0.14), 0 6px 10px 0 rgba(0,0,0,0.1), 0 1px 18px 0 rgba(0,0,0,0.08)',
      // Readable secondary text (kept from previous dark fix)
      ...(isDark
        ? {
            colorTextSecondary: 'rgba(255, 255, 255, 0.85)',
            colorTextTertiary: 'rgba(255, 255, 255, 0.65)',
            colorTextQuaternary: 'rgba(255, 255, 255, 0.55)',
          }
        : {}),
    },
    components: {
      Button: {
        borderRadius: 8,
        controlHeight: 36,
        controlHeightLG: 42,
        controlHeightSM: 30,
        paddingContentHorizontal: 16,
        fontWeight: 500,
        // Contained primary — soft Material shadow (not antd default hard shadow)
        primaryShadow: isDark
          ? '0 1px 3px rgba(0,0,0,0.5), 0 1px 2px rgba(0,0,0,0.4)'
          : '0 1px 3px rgba(0,0,0,0.18), 0 1px 2px rgba(0,0,0,0.12)',
        defaultShadow: 'none',
        dangerShadow: isDark
          ? '0 1px 3px rgba(0,0,0,0.5)'
          : '0 1px 3px rgba(0,0,0,0.16)',
      },
      Card: {
        borderRadiusLG: 12,
        paddingLG: 20,
      },
      Input: {
        borderRadius: 8,
        controlHeight: 40,
        paddingBlock: 8,
      },
      InputNumber: {
        borderRadius: 8,
        controlHeight: 40,
      },
      Select: {
        borderRadius: 8,
        controlHeight: 40,
      },
      DatePicker: {
        borderRadius: 8,
        controlHeight: 40,
      },
      Modal: {
        borderRadiusLG: 12,
      },
      Drawer: {
        // no borderRadius token in all versions — skip if unsupported
      },
      Table: {
        borderRadius: 12,
        headerBorderRadius: 12,
      },
      Tag: {
        borderRadiusSM: 6,
      },
      Menu: {
        itemBorderRadius: 8,
        itemMarginInline: 8,
        itemHeight: 44,
      },
      Tabs: {
        titleFontSize: 14,
        horizontalItemPadding: '12px 16px',
      },
      Message: {
        borderRadiusLG: 8,
      },
      Notification: {
        borderRadiusLG: 12,
      },
      Tooltip: {
        borderRadius: 6,
      },
      Switch: {
        // slightly material track
      },
      Segmented: {
        borderRadius: 8,
        itemSelectedBg: isDark ? 'rgba(255,255,255,0.12)' : undefined,
      },
    },
  };
}
