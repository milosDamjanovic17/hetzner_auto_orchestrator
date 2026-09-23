import './style.css';
import './app.css';

import {AddContext, Contexts, DeleteContext, Init, Status, UseContext} from '../wailsjs/go/main/App';
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

// The open tab: null is Contexts, otherwise one resource type.
let current: ResourceView<any> | null = null;

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
    const tab = (text: string, view: ResourceView<any> | null) => {
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
    tabsEl.replaceChildren(tab('Contexts', null), ...resourceViews.map(v => tab(v.title, v)));
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
        resourceEl.hidden = !status.initialized || current === null;
        if (!status.initialized) {
            return;
        }

        if (current === null) {
            await showContexts(gen);
        } else {
            await showResource(current, status.active, gen);
        }
    } catch (err) {
        if (gen !== generation) {
            return;
        }
        resSource.textContent = '';
        showError(err);
    }
}

document.getElementById('refresh')!.addEventListener('click', () => {
    clearMessages();
    refresh();
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
