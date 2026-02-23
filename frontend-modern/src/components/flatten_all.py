import os
import re

def process_directory(directory):
    for root, _, files in os.walk(directory):
        if 'node_modules' in root:
            continue
        for file in files:
            if file.endswith('.tsx') or file.endswith('.ts') or file.endswith('.css'):
                file_path = os.path.join(root, file)
                process_file(file_path)

def process_file(file_path):
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            content = f.read()

        original_content = content

        # Flatten fractional opacities exactly. Look behind for space or quote to ensure it's a class start
        # e.g text-blue-500/80 -> text-blue-500
        # Wait, the escaping of `/` in TSX templates might cause issues. 
        # So we can match `([a-zA-Z0-9:-]+)\\?/[1-9]0\b` instead, which handles `/` or `\\/`
        new_content = re.sub(r'([a-zA-Z0-9:-]+)/[1-9]0\b', r'\1', content)
        new_content = re.sub(r'([a-zA-Z0-9:-]+)\\\/[1-9]0\b', r'\1', new_content)
        
        # Remove backdrop-blur exactly where it acts as a standalone class word
        # Needs to match word boundaries that aren't inside another class name like `my-backdrop-blur`
        new_content = re.sub(r'(?<=\s|\"|\'|\`)backdrop-blur(?:-[a-z0-9]+)?(?=\s|\"|\'|\`)', '', new_content)

        # Ensure we do not accidentally match parts of urls or comments
        # But tailwind classes don't usually conflict there too much.
        
        # Replace shadows
        new_content = re.sub(r'(?<=\s|\"|\'|\`)shadow-2xl(?=\s|\"|\'|\`)', 'shadow-sm', new_content)
        new_content = re.sub(r'(?<=\s|\"|\'|\`)shadow-xl(?=\s|\"|\'|\`)', 'shadow-sm', new_content)
        new_content = re.sub(r'(?<=\s|\"|\'|\`)shadow-lg(?=\s|\"|\'|\`)', 'shadow-sm', new_content)

        # Replace roundings
        new_content = re.sub(r'(?<=\s|\"|\'|\`)rounded-2xl(?=\s|\"|\'|\`)', 'rounded-md', new_content)
        new_content = re.sub(r'(?<=\s|\"|\'|\`)rounded-xl(?=\s|\"|\'|\`)', 'rounded-md', new_content)
        new_content = re.sub(r'(?<=\s|\"|\'|\`)rounded-lg(?=\s|\"|\'|\`)', 'rounded-md', new_content)

        if content != new_content:
            with open(file_path, 'w', encoding='utf-8') as f:
                f.write(new_content)
            print(f"Flattened {file_path}")
            
    except Exception as e:
        print(f"Error processing {file_path}: {e}")

if __name__ == "__main__":
    src_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    process_directory(src_dir)
