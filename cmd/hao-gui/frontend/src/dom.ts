// Small DOM helpers shared by the views. Text always goes in through
// textContent, never innerHTML, so names from Hetzner cannot inject markup.

export function button(text: string, onClick: () => void, extraClass = ''): HTMLButtonElement {
    const b = document.createElement('button');
    b.className = `btn small ${extraClass}`.trim();
    b.textContent = text;
    b.addEventListener('click', onClick);
    return b;
}

export function text<K extends keyof HTMLElementTagNameMap>(tag: K, content: string, className = ''): HTMLElementTagNameMap[K] {
    const e = document.createElement(tag);
    e.textContent = content;
    if (className) {
        e.className = className;
    }
    return e;
}

export function message(err: unknown): string {
    return err instanceof Error ? err.message : String(err);
}
