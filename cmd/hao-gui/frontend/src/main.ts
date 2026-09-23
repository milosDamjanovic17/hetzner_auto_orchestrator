import './style.css';
import './app.css';

import {
    AddContext, ConsoleURL, Contexts, DeleteContext, Init, Preflight, Status, UseContext,
} from '../wailsjs/go/main/App';
import {preflight} from '../wailsjs/go/models';
import {BrowserOpenURL} from '../wailsjs/runtime/runtime';
import {ResourceView, resourceViews} from './resources';

// The page only displays what Go returns. Messages are written in Go; the
// frontend shows them and does not interpret them. Names are always inserted
// with textContent, never innerHTML, so a context name cannot inject markup.

document.querySelector('#app')!.innerHTML = `
    <header>
      <div class="label">Active project</div>
      <div id="active" class="active"></div>
      <button id="refresh" class="btn">Refresh</button>
    </header>
    <nav id="tabs" class="tabs" hidden></nav>
    <div id="env-banner" class="banner" hidden>
      HCLOUD_TOKEN is set in the environment and is ignored; hao uses its own encrypted store.
    </div>
    <div id="error" class="error" hidden></div>
    <div id="notice" class="notice" hidden></div>
    <main>
      <section id="setup" hidden>
        <p class="muted">No encrypted store yet. It holds your project tokens, encrypted with a key kept in the OS keychain.</p>
        <button id="init" class="btn">Create encrypted store</button>
      </section>
      <section id="manage" hidden>
        <h2>Contexts</h2>
        <p id="empty" class="muted" hidden>No contexts yet. Add one below.</p>
        <ul id="contexts" class="contexts"></ul>

        <h2>Add a context</h2>
        <form id="add" class="add-form" autocomplete="off">
          <input id="add-name" type="text" placeholder="name" spellcheck="false">
          <input id="add-token" type="password" placeholder="Read & Write API token" spellcheck="false">
          <button id="add-submit" class="btn" type="submit">Add</button>
        </form>
        <p class="muted">The token is checked with Hetzner before anything is saved.</p>
      </section>
      <section id="resource" hidden>
        <h2 id="res-title"></h2>
        <p id="res-source" class="muted"></p>
        <table id="res-table" class="table" hidden>
          <thead><tr id="res-head"></tr></thead>
          <tbody id="res-body"></tbody>
        </table>
      </section>
      <section id="preflight" hidden>
        <h2>Preflight</h2>
        <form id="pf-form" class="add-form" autocomplete="off">
          <input id="pf-query" class="wide" type="text" spellcheck="false"
                 placeholder="fsn1, nbg1, hel1  or  ccx13 ash  (empty: everything)">
          <button class="btn" type="submit">Check</button>
        </form>
        <p id="pf-source" class="muted"></p>
        <div id="pf-result"></div>
        <p class="muted">Availability only - not a quota check; project limits can still fail a create.</p>
      </section>
      <section id="console" hidden>
        <h2>API tokens and members</h2>
        <p class="muted">API tokens and project members cannot be listed or managed through the Hetzner API.</p>
        <button id="open-console" class="btn">Open Hetzner Console</button>
        <p id="console-url" class="muted"></p>
      </section>
    </main>
`;

const activeEl = document.getElementById('active')!;
const tabsEl = document.getElementById('tabs')!;
const envBanner = document.getElementById('env-banner')!;
const errorEl = document.getElementById('error')!;
const noticeEl = document.getElementById('notice')!;
const setupEl = document.getElementById('setup')!;
const manageEl = document.getElementById('manage')!;
const emptyEl = document.getElementById('empty')!;
const listEl = document.getElementById('contexts')!;
const addForm = document.getElementById('add') as HTMLFormElement;
const addName = document.getElementById('add-name') as HTMLInputElement;
const addToken = document.getElementById('add-token') as HTMLInputElement;
const addSubmit = document.getElementById('add-submit') as HTMLButtonElement;
const resourceEl = document.getElementById('resource')!;
const resTitle = document.getElementById('res-title')!;
const resSource = document.getElementById('res-source')!;
const resTable = document.getElementById('res-table')!;
const resHead = document.getElementById('res-head')!;
const resBody = document.getElementById('res-body')!;
const preflightEl = document.getElementById('preflight')!;
const pfForm = document.getElementById('pf-form') as HTMLFormElement;
const pfQuery = document.getElementById('pf-query') as HTMLInputElement;
const pfSource = document.getElementById('pf-source')!;
const pfResult = document.getElementById('pf-result')!;
const consoleEl = document.getElementById('console')!;
const consoleUrlEl = document.getElementById('console-url')!;

// The open tab: null is Contexts, a string is one of the two fixed panels,
// otherwise one resource type.
type Tab = ResourceView<any> | 'preflight' | 'console' | null;
let current: Tab = null;

// Bumped on every refresh. A slow response from an earlier refresh (say, the
// tab clicked before this one) sees a newer number and does not render.
let generation = 0;

function showError(err: unknown) {
    errorEl.textContent = err instanceof Error ? err.message : String(err);
    errorEl.hidden = false;
}

function clearMessages() {
    errorEl.hidden = true;
    noticeEl.hidden = true;
}

// act runs one user action, shows its error or notice, then reloads
// everything from Go: the page never guesses what the store now holds.
async function act(action: () => Promise<void>, notice: string) {
    clearMessages();
    try {
        await action();
        noticeEl.textContent = notice;
        noticeEl.hidden = false;
    } catch (err) {
        showError(err);
    }
    await refresh();
}

function button(text: string, onClick: () => void, extraClass = ''): HTMLButtonElement {
    const b = document.createElement('button');
    b.className = `btn small ${extraClass}`.trim();
    b.textContent = text;
    b.addEventListener('click', onClick);
    return b;
}

// confirmDelete swaps the row's buttons for a question that names the
// context. The store may hold the only copy of its token.
function confirmDelete(actions: HTMLElement, name: string) {
    const question = document.createElement('span');
    question.textContent = `Delete ${name}? Its token is removed from hao's store, but still works at Hetzner.`;
    actions.replaceChildren(
        question,
        button('Delete', () => act(() => DeleteContext(name),
            `Deleted ${name}. Revoke its token in the Hetzner Console if you no longer need it.`), 'danger'),
        button('Cancel', () => refresh()),
    );
}

function renderContext(name: string, active: boolean): HTMLLIElement {
    const li = document.createElement('li');
    const label = document.createElement('span');
    label.className = 'name';
    label.textContent = name;
    li.append(label);
    if (active) {
        li.classList.add('is-active');
        const tag = document.createElement('span');
        tag.className = 'tag';
        tag.textContent = 'active';
        label.append(' ', tag);
    }

    const actions = document.createElement('span');
    actions.className = 'actions';
    if (!active) {
        actions.append(button('Use', () => act(() => UseContext(name), `Active project is now ${name}.`)));
    }
    actions.append(button('Delete', () => confirmDelete(actions, name)));
    li.append(actions);
    return li;
}

function renderTabs() {
    const tab = (text: string, view: Tab) => {
        const b = document.createElement('button');
        b.className = 'tab';
        b.textContent = text;
        b.classList.toggle('selected', view === current);
        b.addEventListener('click', () => {
            current = view;
            clearMessages();
            refresh();
        });
        return b;
    };
    tabsEl.replaceChildren(
        tab('Contexts', null),
        ...resourceViews.map(v => tab(v.title, v)),
        tab('Preflight', 'preflight'),
        tab('Tokens & Members', 'console'),
    );
}

async function showContexts(gen: number) {
    const contexts = await Contexts();
    if (gen !== generation) {
        return;
    }
    listEl.replaceChildren();
    emptyEl.hidden = contexts.length > 0;
    for (const c of contexts) {
        listEl.append(renderContext(c.name, c.active));
    }
}

async function showResource(view: ResourceView<any>, active: string, gen: number) {
    resTitle.textContent = view.title;
    resTable.hidden = true;
    if (!active) {
        resSource.textContent = 'No active project. Pick one under Contexts.';
        return;
    }
    resSource.textContent = 'Loading…';
    const listing = await view.fetch();
    if (gen !== generation) {
        return;
    }
    // The project named here comes from the response, not from the header:
    // it is the one the data was actually fetched with.
    if (listing.items.length === 0) {
        resSource.textContent = `No ${view.plural} in project ${listing.context}.`;
        return;
    }
    resSource.textContent = `Project ${listing.context}: ${listing.items.length} ${view.plural}.`;
    resHead.replaceChildren(...view.columns.map(c => {
        const th = document.createElement('th');
        th.textContent = c.title;
        return th;
    }));
    resBody.replaceChildren(...listing.items.map(item => {
        const tr = document.createElement('tr');
        for (const c of view.columns) {
            const td = document.createElement('td');
            td.textContent = String(c.value(item));
            tr.append(td);
        }
        return tr;
    }));
    resTable.hidden = false;
}

// availabilityTable shows server types with the columns `hao preflight` prints.
function availabilityTable(rows: preflight.Availability[]): HTMLTableElement {
    const table = document.createElement('table');
    table.className = 'table';
    const head = table.createTHead().insertRow();
    for (const title of ['Type', 'Location', 'State', 'Cores', 'RAM GB', 'Disk GB', 'Arch', 'Notes']) {
        const th = document.createElement('th');
        th.textContent = title;
        head.append(th);
    }
    const body = table.createTBody();
    for (const a of rows) {
        const notes = [];
        if (a.recommended) {
            notes.push('recommended');
        }
        if (a.deprecated) {
            notes.push('deprecated');
        }
        const tr = body.insertRow();
        for (const v of [a.server_type, a.location, a.available ? 'available' : 'unavailable',
            a.cores, a.memory_gb.toFixed(1), a.disk_gb, a.architecture, notes.join(',')]) {
            tr.insertCell().textContent = String(v);
        }
    }
    return table;
}

// showPreflight runs the query in the field, the way a resource tab fetches
// its list: on opening the tab, on Check, and on Refresh. What the words mean
// is decided in Go, shared with the CLI.
async function showPreflight(active: string, gen: number) {
    pfResult.replaceChildren();
    if (!active) {
        pfSource.textContent = 'No active project. Pick one under Contexts.';
        return;
    }
    pfSource.textContent = 'Checking…';
    const res = await Preflight(pfQuery.value);
    if (gen !== generation) {
        return;
    }
    const ans = res.answer;
    pfSource.textContent = `Project ${res.context}:`;
    switch (ans.mode) {
    case 'all':
        pfResult.append(availabilityTable(ans.all));
        break;
    case 'lookup': {
        const p = document.createElement('p');
        p.className = 'answer';
        p.textContent = `${ans.serverType} in ${ans.location}: ${ans.available ? 'available' : 'not available right now'}`;
        pfResult.append(p);
        break;
    }
    case 'locations':
        for (const g of ans.groups) {
            const h = document.createElement('h3');
            h.textContent = `${g.location} (${g.city}): ${g.available.length} server types available`;
            pfResult.append(h);
            if (g.available.length > 0) {
                pfResult.append(availabilityTable(g.available));
            }
        }
        break;
    }
}

async function showConsole(gen: number) {
    const url = await ConsoleURL();
    if (gen !== generation) {
        return;
    }
    consoleUrlEl.textContent = url;
}

async function refresh() {
    const gen = ++generation;
    renderTabs();
    let status;
    try {
        status = await Status();
    } catch (err) {
        if (gen === generation) {
            activeEl.textContent = '?';
            showError(err);
        }
        return;
    }
    if (gen !== generation) {
        return;
    }
    // A failed listing below leaves the header alone: it is still correct.
    try {
        envBanner.hidden = !status.envTokenIgnored;
        activeEl.textContent = status.active || 'none';
        activeEl.classList.toggle('none', !status.active);
        setupEl.hidden = status.initialized;
        tabsEl.hidden = !status.initialized;
        manageEl.hidden = !status.initialized || current !== null;
        resourceEl.hidden = !status.initialized || current === null || typeof current === 'string';
        preflightEl.hidden = !status.initialized || current !== 'preflight';
        consoleEl.hidden = !status.initialized || current !== 'console';
        if (!status.initialized) {
            return;
        }

        if (current === null) {
            await showContexts(gen);
        } else if (current === 'preflight') {
            await showPreflight(status.active, gen);
        } else if (current === 'console') {
            await showConsole(gen);
        } else {
            await showResource(current, status.active, gen);
        }
    } catch (err) {
        if (gen !== generation) {
            return;
        }
        resSource.textContent = '';
        pfSource.textContent = '';
        showError(err);
    }
}

document.getElementById('refresh')!.addEventListener('click', () => {
    clearMessages();
    refresh();
});

pfForm.addEventListener('submit', (e) => {
    e.preventDefault();
    clearMessages();
    refresh();
});

// Opened in the system browser: the Console needs the user's own login,
// which does not belong inside this window.
document.getElementById('open-console')!.addEventListener('click', async () => {
    BrowserOpenURL(await ConsoleURL());
});

document.getElementById('init')!.addEventListener('click', () =>
    act(() => Init(), 'Encrypted store created.'));

addForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = addName.value.trim();
    // Disabled while Hetzner checks the token, so a second click cannot
    // send it twice.
    addSubmit.disabled = true;
    addSubmit.textContent = 'Checking…';
    await act(async () => {
        await AddContext(name, addToken.value);
        // Cleared only on success: after a network error the user can retry
        // without pasting again.
        addName.value = '';
        addToken.value = '';
    }, `Added ${name} (token validated).`);
    addSubmit.disabled = false;
    addSubmit.textContent = 'Add';
});

refresh();
