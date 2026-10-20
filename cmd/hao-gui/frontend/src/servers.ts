import {
    BackupServer, CreateOptions, CreateServer, DeleteServer, Estimate, ImageCost, PowerServer,
    ProtectServer, SnapshotServer, UnprotectServer,
} from '../wailsjs/go/main/App';
import {hetzner, main, service} from '../wailsjs/go/models';
import {button, message, text} from './dom';

// Server writes (phase-2b Step 4). Every guard runs in Go. What the page
// checks here (a typed name matching, a key picked, backups on) only enables
// buttons; Go checks again. Every write passes the project named by the
// response the page rendered, so a switch in the CLI meanwhile is refused.

// Act is main.ts's act: run one write, show its error or notice, reload from
// Go. A write resolves to Go's note when still running at Hetzner, else "".
export type Act = (action: () => Promise<string | void>, notice: string) => Promise<void>;

const working = 'Working… Hetzner can take up to 75 s.';

// serverActions is the row's Actions button. It opens a full-width row
// under the server with that server's actions; confirmations replace it.
export function serverActions(s: hetzner.Server, project: string, act: Act): HTMLElement {
    const detail = document.createElement('tr');
    detail.className = 'detail';
    const cell = detail.insertCell();
    const panel = document.createElement('div');
    panel.className = 'server-actions';
    cell.append(panel);

    const toggle = button('Actions', () => {
        if (detail.isConnected) {
            detail.remove();
            return;
        }
        const row = toggle.closest('tr')!;
        cell.colSpan = row.cells.length;
        row.after(detail);
        menu();
    });

    const run = (action: () => Promise<string>, notice: string) => {
        panel.replaceChildren(text('span', working));
        return act(action, notice);
    };

    // ask is the plain confirm: it names the server and the project.
    const ask = (question: string, label: string, action: () => Promise<string>, notice: string) =>
        panel.replaceChildren(
            text('p', question),
            button(label, () => run(action, notice), 'danger'),
            button('Cancel', menu),
        );

    // typed confirms delete and protection off: the button enables only when
    // the field holds the exact name. Go compares it again, by server ID.
    const typed = (question: string, label: string, action: (name: string) => Promise<string>, notice: string) => {
        const input = document.createElement('input');
        input.type = 'text';
        input.spellcheck = false;
        input.placeholder = 'type the server name';
        const go = button(label, () => run(() => action(input.value), notice), 'danger');
        go.disabled = true;
        input.addEventListener('input', () => go.disabled = input.value !== s.name);
        panel.replaceChildren(text('p', question), input, go, button('Cancel', menu));
        input.focus();
    };

    // withCost fetches live prices before a backup or snapshot confirm.
    const withCost = async (show: (c: service.ImageCost) => void) => {
        panel.replaceChildren(text('span', 'Fetching prices…'));
        try {
            show(await ImageCost(s.disk_gb));
        } catch (err) {
            panel.replaceChildren(text('p', message(err), 'inline-error'), button('Back', menu));
        }
    };

    const power = (verb: string) => () => PowerServer(project, s.id, verb);

    function menu() {
        const state = [
            `status ${s.status}`,
            s.delete_protected ? 'protection on' : 'protection off',
            s.backups ? 'backups on' : 'backups off',
            s.managed ? 'created by hao' : 'not created by hao',
        ].join(' · ');
        const buttons: HTMLButtonElement[] = [];

        if (s.status === 'off') {
            buttons.push(button('Power on', () => run(power('poweron'), `Powered on ${s.name}.`)));
        } else if (s.status === 'running') {
            buttons.push(
                button('Reboot', () => ask(`Reboot ${s.name} in project ${project}?`,
                    'Reboot', power('reboot'), `Rebooted ${s.name}.`)),
                button('Shut down', () => ask(`Shut down ${s.name} in project ${project}? The OS shuts down cleanly.`,
                    'Shut down', power('shutdown'), `Shut down ${s.name}.`)),
                button('Power off', () => ask(`Power off ${s.name} in project ${project}? This cuts power like pulling the plug; unsaved data is lost.`,
                    'Power off', power('poweroff'), `Powered off ${s.name}.`)),
            );
        }

        const backup = button('Backup', () => withCost(c => ask(
            `Back up ${s.name} in project ${project} now? Backups are billed through the backup option: `
            + `${c.backup_percent || '(unknown)'}% of the server price (Hetzner's live price list). `
            + 'Hetzner keeps up to 7 backups; when all are taken, the oldest is deleted.',
            'Create backup', () => BackupServer(project, s.id), `Backup of ${s.name} created.`)));
        if (!s.backups) {
            backup.disabled = true;
            backup.title = 'Backups are off for this server; enable them in the Console.';
        }
        buttons.push(backup);

        buttons.push(button('Snapshot', () => withCost(c => {
            const description = document.createElement('input');
            description.type = 'text';
            description.placeholder = 'description (optional)';
            panel.replaceChildren(
                text('p', `Snapshot ${s.name} in project ${project}? Snapshots cost € ${c.per_gb_month} per GB per month: `
                    + `at most € ${c.snapshot_max} / month for this server's ${s.disk_gb} GB disk. Kept until deleted in the Console.`),
                description,
                button('Create snapshot', () => run(() => SnapshotServer(project, s.id, description.value),
                    `Snapshot of ${s.name} created.`), 'danger'),
                button('Cancel', menu),
            );
        })));

        if (s.delete_protected) {
            buttons.push(button('Unprotect', () => typed(
                `Turn off delete and rebuild protection for ${s.name} in project ${project}? Type the server name to confirm.`,
                'Turn protection off', name => UnprotectServer(project, s.id, name), `Protection off for ${s.name}.`)));
        } else {
            buttons.push(button('Protect', () => run(() => ProtectServer(project, s.id),
                `Delete and rebuild protection on for ${s.name}.`)));
        }

        const del = button('Delete', () => typed(
            `Delete ${s.name} (id ${s.id}) in project ${project}? This cannot be undone. Type the server name to confirm.`,
            'Delete server', name => DeleteServer(project, s.id, name), `Deleted ${s.name}.`), 'danger');
        if (!s.managed) {
            del.disabled = true;
            del.title = 'Not created by hao - delete it in the Console.';
        } else if (s.delete_protected) {
            del.disabled = true;
            del.title = 'Delete protection is on; turn it off first.';
        }
        buttons.push(del, button('Close', () => detail.remove()));

        panel.replaceChildren(text('p', state, 'muted'), ...buttons);
    }

    return toggle;
}

// fetchedAt shows when the prices were fetched, in local time.
function fetchedAt(t: string): string {
    const d = new Date(t);
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// costLines is the cost check, the same facts `hao server create` prints.
function costLines(e: service.Estimate): HTMLElement {
    const box = document.createElement('div');
    box.className = 'cost';
    box.append(
        text('p', `Creating this server costs money. Prices fetched from Hetzner at ${fetchedAt(e.fetched)}; Hetzner may change them.`),
        text('p', `${e.type} in ${e.location}: € ${e.hourly} / hour, capped at € ${e.monthly} / month`),
        text('p', `Primary IPv4: € ${e.ipv4} / month`),
        text('p', `Estimate, 30 days 24/7 (720 h): € ${e.total}`, 'total'),
        text('p', 'Billed while the server EXISTS, even when powered off. Not included: traffic over the included amount, backups, snapshots.', 'muted'),
    );
    return box;
}

function select(): HTMLSelectElement {
    return document.createElement('select');
}

function fill(sel: HTMLSelectElement, options: [value: string, label: string][]) {
    sel.replaceChildren(...options.map(([value, label]) => {
        const o = document.createElement('option');
        o.value = value;
        o.textContent = label;
        return o;
    }));
}

// createPanel is the "Create server" button above the table. The form's
// choices load only when it is opened: four API calls.
export function createPanel(_project: string, act: Act): HTMLElement {
    const box = document.createElement('div');
    box.className = 'create';
    const closed = () => box.replaceChildren(button('Create server', load));

    async function load() {
        box.replaceChildren(text('span', 'Loading server types, images and SSH keys…'));
        try {
            form(await CreateOptions());
        } catch (err) {
            box.replaceChildren(text('p', message(err), 'inline-error'), button('Close', closed));
        }
    }

    function form(o: main.CreateOptions) {
        const heading = text('h3', `New server in project ${o.context}`);
        if (o.ssh_keys.length === 0) {
            box.replaceChildren(heading,
                text('p', `No SSH keys in project ${o.context}. Add one in the Hetzner Console first: `
                    + 'hao creates servers with an SSH key only, and never creates keys.'),
                button('Close', closed));
            return;
        }
        if (o.types.length === 0) {
            box.replaceChildren(heading, text('p', 'No server type is available in any location right now.'),
                button('Close', closed));
            return;
        }

        const fields = document.createElement('fieldset');
        const name = document.createElement('input');
        name.type = 'text';
        name.spellcheck = false;
        name.placeholder = 'server name';
        const location = select();
        const type = select();
        const image = select();

        const cities = new Map<string, string>();
        for (const t of o.types) {
            cities.set(t.location, t.city);
        }
        fill(location, [...cities].sort((a, b) => a[0].localeCompare(b[0])).map(([loc, city]) => [loc, `${loc} (${city})`]));

        // The type picks the image's architecture: x86 images for x86 types.
        const pickTypes = () => fill(type, o.types.filter(t => t.location === location.value).map(t =>
            [t.server_type, `${t.server_type} - ${t.cores} vCPU, ${t.memory_gb} GB RAM, ${t.disk_gb} GB disk, ${t.architecture}`]));
        const pickImages = () => {
            const arch = o.types.find(t => t.server_type === type.value)?.architecture;
            fill(image, o.images.filter(i => i.architecture === arch).map(i =>
                [i.name, i.description ? `${i.name} (${i.description})` : i.name]));
        };

        const keys = o.ssh_keys.map(k => {
            const label = document.createElement('label');
            const check = document.createElement('input');
            check.type = 'checkbox';
            check.value = k.name;
            label.append(check, ` ${k.name}`);
            return {label, check};
        });
        const picked = () => keys.filter(k => k.check.checked).map(k => k.check.value);

        // The cost box follows type and location. A slower answer for an
        // earlier pick sees a newer number and is dropped.
        const cost = document.createElement('div');
        let costGen = 0;
        const updateCost = async () => {
            const gen = ++costGen;
            cost.replaceChildren(text('p', 'Fetching prices…', 'muted'));
            try {
                const e = await Estimate(type.value, location.value);
                if (gen === costGen) {
                    cost.replaceChildren(costLines(e));
                }
            } catch (err) {
                if (gen === costGen) {
                    cost.replaceChildren(text('p', message(err), 'inline-error'));
                }
            }
        };

        const review = button('Review cost and create', () => confirm());
        const ready = () => review.disabled = !name.value || picked().length === 0 || !type.value || !image.value;
        name.addEventListener('input', ready);
        keys.forEach(k => k.check.addEventListener('change', ready));
        location.addEventListener('change', () => {
            pickTypes();
            pickImages();
            ready();
            updateCost();
        });
        type.addEventListener('change', () => {
            pickImages();
            ready();
            updateCost();
        });

        const confirmBox = document.createElement('div');

        // confirm fetches the prices again at submit (phase-2b §6) and locks
        // the form, so what is created is exactly what the confirm shows.
        async function confirm() {
            fields.disabled = true;
            review.hidden = true;
            confirmBox.replaceChildren(text('p', 'Fetching prices…', 'muted'));
            const back = () => {
                fields.disabled = false;
                review.hidden = false;
                confirmBox.replaceChildren();
            };
            let e: service.Estimate;
            try {
                e = await Estimate(type.value, location.value);
            } catch (err) {
                confirmBox.replaceChildren(text('p', message(err), 'inline-error'), button('Back', back));
                return;
            }
            const n = name.value, t = type.value, l = location.value, i = image.value, k = picked();
            const create = button('Create server', () => {
                create.disabled = true; // a second click would create a second server
                confirmBox.replaceChildren(text('span', working));
                act(() => CreateServer(o.context, n, t, l, i, k), `Created ${n} in project ${o.context}.`);
            }, 'danger');
            confirmBox.replaceChildren(
                costLines(e),
                text('p', `Create "${n}" (${t}, ${l}, ${i}, keys ${k.join(', ')}) in project ${e.context}?`),
                create,
                button('Back', back),
            );
        }

        const keyList = document.createElement('div');
        keyList.className = 'keys';
        keyList.append(...keys.map(k => k.label));
        fields.append(
            text('label', 'Name'), name,
            text('label', 'Location'), location,
            text('label', 'Type'), type,
            text('label', 'Image'), image,
            text('label', 'SSH keys'), keyList,
        );

        pickTypes();
        pickImages();
        ready();
        updateCost();
        box.replaceChildren(heading, fields, cost, review, button('Cancel', closed), confirmBox);
    }

    closed();
    return box;
}
