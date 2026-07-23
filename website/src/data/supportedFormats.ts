// Formats that support both Syntax and Schema validation
export const SCHEMA_FORMATS = [
  'JSON', 'JSONC', 'YAML', 'TOML', 'XML', 'TOON', 'SARIF'
];

// Formats that support only Syntax validation
export const SYNTAX_FORMATS = [
  'HCL', 'INI', 'HOCON', 'ENV', 'CSV', 'Properties', 
  'EDITORCONFIG', 'Justfile', 'KDL', 'CUE', 'PList'
];

// Master list of all supported configuration file formats
export const SUPPORTED_FORMATS = [...SCHEMA_FORMATS, ...SYNTAX_FORMATS];

// Formats an array of strings into a readable comma-separated list (e.g., "A, B, and C").
export function formatList(items: string[]): string {
  if (items.length === 0) return '';
  if (items.length === 1) return items[0] + '.';
  const last = items[items.length - 1];
  const rest = items.slice(0, -1).join(', ');
  return `${rest}, and ${last}.`;
}