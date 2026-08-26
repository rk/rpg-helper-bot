import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Components } from "react-markdown";
import { ChatSource } from "./api";
import { SourceFlyover } from "./SourceFlyover";

interface MarkdownMessageProps {
  content: string;
  sources?: ChatSource[];
}

function linkifyCitations(content: string, sourceCount: number): string {
  if (sourceCount === 0) return content;
  return content.replace(/\[(\d+)\]/g, (match, num) => {
    const n = parseInt(num, 10);
    if (n >= 1 && n <= sourceCount) {
      return `[${num}](#source-${n})`;
    }
    return match;
  });
}

export function MarkdownMessage({ content, sources = [] }: MarkdownMessageProps) {
  const prepared = linkifyCitations(content, sources.length);

  const components: Components = {
    a({ href, children }) {
      const sourceMatch = href?.match(/^#source-(\d+)$/);
      if (sourceMatch) {
        const idx = parseInt(sourceMatch[1], 10) - 1;
        const source = sources[idx];
        if (source) {
          return (
            <SourceFlyover source={source}>
              <a href={href} className="citation-link">
                {children}
              </a>
            </SourceFlyover>
          );
        }
      }
      if (href?.startsWith("http")) {
        return (
          <a href={href} target="_blank" rel="noopener noreferrer">
            {children}
          </a>
        );
      }
      return <a href={href}>{children}</a>;
    },
  };

  return (
    <div className="markdown-body">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {prepared}
      </ReactMarkdown>
    </div>
  );
}
