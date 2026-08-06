import os
import re

for root, _, files in os.walk('templates/testdata'):
    for file in files:
        if file.endswith('.txtar'):
            path = os.path.join(root, file)
            with open(path, 'r') as f:
                content = f.read()

            # Use regex to find and remove duplicated ConstructorMethodName lines
            # A line is duplicated if it is immediately followed by identical lines
            lines = content.split('\n')
            new_lines = []
            prev_line = None
            for line in lines:
                if 'ConstructorMethodName' in line:
                    if line == prev_line:
                        continue
                new_lines.append(line)
                prev_line = line

            with open(path, 'w') as f:
                f.write('\n'.join(new_lines))
