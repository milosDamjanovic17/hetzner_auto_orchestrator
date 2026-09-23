import './style.css';
import './app.css';

import {Contexts, Status} from '../wailsjs/go/main/App';

// The page only displays what Go returns. Messages are written in Go; the
// frontend shows them and does not interpret them. Names are always inserted
// with textContent, never innerHTML, so a context name cannot inject markup.

document.querySelector('#app')!.innerHTML = `
    <header>
      <div class="label">Active project</div>
      <div id="active" class="active"></div>
      <button id="refresh" class="btn">Refresh</button>
    </header>
    <div id="env-banner" class="banner" hidden>
      HCLOUD_TOKEN is set in the environment and is ignored; hao uses its own encrypted store.
    </div>
    <div id="error" class="error" hidden></div>
    <main>
      <h2>Contexts</h2>
      <p id="empty" class="muted" hidden></p>
      <ul id="contexts" class="contexts"></ul>
    </main>
`;

const activeEl = document.getElementById('active')!;
const envBanner = document.getElementById('env-banner')!;
const errorEl = document.getElementById('error')!;
const emptyEl = document.getElementById('empty')!;
const listEl = document.getElementById('contexts')!;

function showError(err: unknown) {
    errorEl.textContent = err instanceof Error ? err.message : String(err);
    errorEl.hidden = false;
}

async function refresh() {
    errorEl.hidden = true;
    listEl.replaceChildren();
    emptyEl.hidden = true;
    try {
        const status = await Status();
        envBanner.hidden = !status.envTokenIgnored;
        activeEl.textContent = status.active || 'none';
        activeEl.classList.toggle('none', !status.active);

        if (!status.initialized) {
            emptyEl.textContent = 'No encrypted store yet. Run `hao init` for now; the GUI gets its own button in Step 3.';
            emptyEl.hidden = false;
            return;
        }

        const contexts = await Contexts();
        if (contexts.length === 0) {
            emptyEl.textContent = 'No contexts yet. Add one with `hao context add <name>` for now.';
            emptyEl.hidden = false;
            return;
        }
        for (const c of contexts) {
            const li = document.createElement('li');
            li.textContent = c.name;
            if (c.active) {
                li.classList.add('is-active');
                const tag = document.createElement('span');
                tag.className = 'tag';
                tag.textContent = 'active';
                li.append(' ', tag);
            }
            listEl.append(li);
        }
    } catch (err) {
        activeEl.textContent = '?';
        showError(err);
    }
}

document.getElementById('refresh')!.addEventListener('click', refresh);
refresh();
