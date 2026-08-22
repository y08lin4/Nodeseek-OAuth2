// naive-ui 品牌主题覆盖（NS 风格：GitHub 绿主色 + 灰字体系 + 6px 圆角）
// 与 src/style.css 的 :root CSS 变量保持一致，一套色板两处引用。
import type { GlobalThemeOverrides } from 'naive-ui'

export const themeOverrides: GlobalThemeOverrides = {
  common: {
    // 主色体系（NS 实测 #2ea44f 系列）
    primaryColor: '#2ea44f',
    primaryColorHover: '#2c974b',
    primaryColorPressed: '#24833c',
    primaryColorSuppl: '#d8edd8',
    // 语义色（GitHub Primer）
    infoColor: '#1976d2',
    infoColorHover: '#1565c0',
    infoColorPressed: '#0d47a1',
    successColor: '#2ea44f',
    successColorHover: '#2c974b',
    successColorPressed: '#1a7f37',
    warningColor: '#ed6c02',
    warningColorHover: '#bf5a00',
    warningColorPressed: '#9a6700',
    errorColor: '#d32f2f',
    errorColorHover: '#b71c1c',
    errorColorPressed: '#9e1a1a',
    // 文本色层级
    textColorBase: '#333',
    textColor1: '#333',
    textColor2: '#555',
    textColor3: '#888',
    // 边框与圆角（统一 6px）
    borderColor: '#e5e7eb',
    borderRadius: '6px',
  },
  Button: {
    borderRadius: '6px',
    borderRadiusMedium: '6px',
    borderRadiusLarge: '6px',
    borderRadiusSmall: '6px',
    fontSizeMedium: '14px',
    fontWeight: '600',
  },
  Card: {
    borderRadius: '6px',
    borderColor: '#e5e7eb',
    paddingMedium: '20px',
    titleFontSizeMedium: '15px',
    titleFontWeight: '600',
  },
  Dialog: {
    borderRadius: '6px',
  },
  Tag: {
    borderRadius: '6px',
    fontWeight: '500',
  },
  Input: {
    borderRadius: '6px',
    heightMedium: '32px',
  },
  Select: {
    borderRadius: '6px',
  },
  Alert: {
    borderRadius: '6px',
  },
  Table: {
    borderRadius: '6px',
    thColor: '#fbfbfb',
    thFontWeight: '600',
  },
  Empty: {
    borderRadius: '6px',
  },
  DatePicker: {
    borderRadius: '6px',
  },
  Popover: {
    borderRadius: '6px',
  },
  DataTable: {
    borderRadius: '6px',
  },
}

