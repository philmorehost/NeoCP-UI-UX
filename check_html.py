import re

with open('neocp/web/static/index.html', 'r') as f:
    content = f.read()

# Simple tag tracker
stack = []
# Updated regex to capture self-closing /
regex = r'<(/?)(\w+)(?:\s+[^>]*?)?(/?)\s*>'

for match in re.finditer(regex, content, re.IGNORECASE):
    is_closing = match.group(1) == '/'
    tag_name = match.group(2).lower()
    is_self_closing = match.group(3) == '/'
    line_no = content.count('\n', 0, match.start()) + 1

    if tag_name in ['input', 'br', 'hr', 'img', 'meta', 'link']:
        continue # known self-closing in HTML

    if is_self_closing:
        continue # explicitly self-closing <tag />

    if not is_closing:
        stack.append((tag_name, line_no))
    else:
        if not stack:
            print(f"Error: Closing tag </{tag_name}> at line {line_no} has no opening tag")
            continue
        last_tag, last_line = stack.pop()
        if last_tag != tag_name:
            print(f"Error: Mismatched tags. Expected </{last_tag}> (from line {last_line}), found </{tag_name}> at line {line_no}")

while stack:
    tag, line = stack.pop()
    print(f"Error: Unclosed tag <{tag}> from line {line}")
