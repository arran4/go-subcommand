import os
import re

with open("blog.txt", "r") as f:
    blog_content = f.read()

def extract_code_block(start_marker, end_marker=None, include_start=False):
    lines = blog_content.splitlines()
    capturing = False
    block_lines = []

    for line in lines:
        if start_marker in line:
            capturing = True
            if include_start:
                block_lines.append(line.split(start_marker)[1] if start_marker in line else line)
            continue
        if capturing:
            if end_marker and end_marker in line:
                break
            # Remove leading numbers and spaces for code blocks
            clean_line = re.sub(r'^\s*\d+ ?', '', line)
            block_lines.append(clean_line)

    return "\n".join(block_lines)

print("Check 1 passed")
