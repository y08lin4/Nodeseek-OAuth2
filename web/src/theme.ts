// naive-ui 品牌主题覆盖（NS 风格：GitHub 绿主色 + 灰字体系 + Pure 直角）
// 与 src/style.css 的 :root CSS 变量保持一致，一套色板两处引用。
import type { GlobalThemeOverrides } from 'naive-ui'

export const themeOverrides: GlobalThemeOverrides = {
  common: {
    // 主色（NS 实测 #2ea44f 系列）
    primaryColor: '#2ea44f',
    primaryColorHover: '#1a7f37',
    primaryColorPressed: '#1a7f37',
    primaryColorSuppl: '#2ea44f',
    // 语义色（GitHub Primer）
    infoColor: '#1f6feb',
    infoColorHover: '#1a55c9',
    infoColorPressed: '#1a55c9',
    successColor: '#1a7f37',
    successColorHover: '#2ea44f',
    successColorPressed: '#0d5e27',
    warningColor: '#9a6700',
    warningColorHover: '#b87b00',
    warningColorPressed: '#7a5200',
    errorColor: '#cf222e',
    errorColorHover: '#e0484f',
    errorColorPressed: '#a81222',
    // 文本色（深灰，NS --dark-color）
    textColorBase: '#333',
    textColor1: '#333',
    textColor2: '#555',
    textColor3: '#888',
    // 边框（Tailwind 灰）
    borderColor: '#e5e7eb',
    // 圆角收紧（Pure 风：直角）
    borderRadius: '4px',
  },
  Button: {
    borderRadius: '4px',
    borderRadiusMedium: '4px',
    borderRadiusLarge: '4px',
    fontWeight: '600',
  },
  Card: {
    borderRadius: '6px',
    borderColor: '#e5e7eb',
  },
  Tag: {
    borderRadius: '4px',
  },
  Input: {
    borderRadius: '4px',
  },
  Alert: {
    borderRadius: '6px',
  },
  Table: {
    borderRadius: '6px',
    thColor: '#fbfbfb',
  },
  Empty: {
    borderRadius: '6px',
  },
  Select: {
    borderRadius: '4px',
  },
  DatePicker: {
    borderRadius: '4px',
  },
  Popover: {
    borderRadius: '6px',
  },
}
