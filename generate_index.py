#!/usr/bin/env python3
import os

def read_file(filepath):
    with open(filepath, 'r', encoding='utf-8') as f:
        return f.read()

def main():
    base_dir = os.path.dirname(os.path.abspath(__file__))
    static_dir = os.path.join(base_dir, 'internal', 'api', 'static')
    
    html_parts = []
    
    html_parts.append(read_file(os.path.join(static_dir, 'index_head.html')))
    html_parts.append("<style>\n")
    html_parts.append(read_file(os.path.join(static_dir, 'css', 'theme.css')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'css', 'layout.css')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'css', 'components.css')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'vendor', 'github-dark.min.css')))
    html_parts.append("\n</style>\n")
    html_parts.append(read_file(os.path.join(static_dir, 'index_body.html')))
    
    html_parts.append("<script>\n")
    html_parts.append(read_file(os.path.join(static_dir, 'vendor', 'marked.min.js')))
    html_parts.append("\n</script>\n")
    html_parts.append("<script>\n")
    html_parts.append(read_file(os.path.join(static_dir, 'vendor', 'highlight.min.js')))
    html_parts.append("\n</script>\n")
    
    html_parts.append("<script>\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'api.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'app.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'overview.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'chat.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'agents.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'workflows.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'workflow-builder.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'channels.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'skills.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'hands.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'scheduler.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'settings.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'usage.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'sessions.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'logs.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'wizard.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'approvals.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'a2a.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'pairing.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'delivery.js')))
    html_parts.append("\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'pages', 'projects.js')))
    html_parts.append("\n</script>\n")
    
    html_parts.append("<script>\n")
    html_parts.append(read_file(os.path.join(static_dir, 'js', 'i18n.js')))
    html_parts.append("\n</script>\n")
    
    html_parts.append("<script>\n")
    html_parts.append(read_file(os.path.join(static_dir, 'vendor', 'alpine.min.js')))
    html_parts.append("\n</script>\n")
    
    html_parts.append("</body></html>")
    
    full_html = "".join(html_parts)
    
    with open(os.path.join(static_dir, 'index.html'), 'w', encoding='utf-8') as f:
        f.write(full_html)
    
    print(f"Generated {os.path.join(static_dir, 'index.html')}")

if __name__ == "__main__":
    main()
