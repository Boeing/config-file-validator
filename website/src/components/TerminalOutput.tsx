import React from 'react';

interface Line {
  text: string;
  status?: 'pass' | 'fail' | 'header' | 'command';
}

interface TerminalOutputProps {
  title?: string;
  lines: Line[];
}

const statusColors: Record<string, string> = {
  pass: '#4caf50',
  fail: '#f44336',
  header: '#9e9e9e',
  command: '#64b5f6',
};

export default function TerminalOutput({title, lines}: TerminalOutputProps) {
  return (
    <div
      style={{
        background: '#1e1e1e',
        borderRadius: '8px',
        padding: '1rem',
        marginBottom: '1.5rem',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
        fontSize: '0.85rem',
        lineHeight: '1.6',
        overflowX: 'auto',
      }}
    >
      {title && (
        <div
          style={{
            color: '#9e9e9e',
            borderBottom: '1px solid #333',
            paddingBottom: '0.5rem',
            marginBottom: '0.5rem',
          }}
        >
          {title}
        </div>
      )}
      {lines.map((line, i) => (
        <div key={i} style={{color: statusColors[line.status] || '#e0e0e0'}}>
          {line.text}
        </div>
      ))}
    </div>
  );
}
