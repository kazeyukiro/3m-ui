import { Grid } from 'antd';

/** True when viewport is below Ant Design `md` breakpoint (768px). */
export function useIsMobile(): boolean {
  const screens = Grid.useBreakpoint();
  // screens.md is undefined during first SSR/hydration tick; treat as desktop until known
  return screens.md === false;
}

export default useIsMobile;
