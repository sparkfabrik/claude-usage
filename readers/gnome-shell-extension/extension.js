/**
 * Claude Code Usage — GNOME Shell panel extension.
 *
 * Calls the Go binary (`claude-usage --status`) periodically and renders
 * the JSON response as panel labels and a dropdown menu. All polling,
 * caching, formatting, and color logic lives in the CLI — the extension
 * is a pure renderer.
 */

import GLib from 'gi://GLib';
import Gio from 'gi://Gio';
import St from 'gi://St';
import Clutter from 'gi://Clutter';
import GObject from 'gi://GObject';

import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import * as PanelMenu from 'resource:///org/gnome/shell/ui/panelMenu.js';
import * as PopupMenu from 'resource:///org/gnome/shell/ui/popupMenu.js';
import { Extension } from 'resource:///org/gnome/shell/extensions/extension.js';

const BINARY_NAME = 'claude-usage';
const REFRESH_SECONDS = 30;
const ANTHROPIC_ORANGE = '#D97706';
const GREY = '#888888';

const ClaudeUsageIndicator = GObject.registerClass(
class ClaudeUsageIndicator extends PanelMenu.Button {
    _init(extPath) {
        super._init(0.5, 'Claude Code Usage', false);

        this._extPath = extPath;
        this._pending = false;

        // Outer box with rounded border in Anthropic orange
        this._box = new St.BoxLayout({
            style_class: 'panel-status-indicators-box',
            x_align: Clutter.ActorAlign.CENTER,
            y_align: Clutter.ActorAlign.CENTER,
            style: `border: 1px solid ${ANTHROPIC_ORANGE}; border-radius: 8px; padding: 2px 10px; margin: 3px 0;`,
        });
        this.add_child(this._box);

        // Sparkle icon
        const iconFile = Gio.File.new_for_path(GLib.build_filenamev([extPath, 'sparkle.svg']));
        const gicon = new Gio.FileIcon({ file: iconFile });
        this._icon = new St.Icon({
            gicon: gicon,
            icon_size: 14,
            y_align: Clutter.ActorAlign.CENTER,
            style: 'padding-right: 6px;',
        });
        this._box.add_child(this._icon);

        // C label
        this._cLabel = new St.Label({
            text: 'C:--',
            y_align: Clutter.ActorAlign.CENTER,
            style: 'font-size: 14px; padding-right: 4px;',
        });
        this._box.add_child(this._cLabel);

        // W label
        this._wLabel = new St.Label({
            text: 'W:--',
            y_align: Clutter.ActorAlign.CENTER,
            style: 'font-size: 14px;',
        });
        this._box.add_child(this._wLabel);

        // Dropdown menu items
        this._menuCurrent = new PopupMenu.PopupMenuItem('Current (5h): --', { reactive: false });
        this.menu.addMenuItem(this._menuCurrent);
        this._menuWeekly = new PopupMenu.PopupMenuItem('Weekly  (7d): --', { reactive: false });
        this.menu.addMenuItem(this._menuWeekly);

        // Error menu item (hidden by default)
        this._menuError = new PopupMenu.PopupMenuItem('', { reactive: false });
        this._menuError.label.set_style('color: #dc3232;');
        this._menuError.visible = false;
        this.menu.addMenuItem(this._menuError);

        // Stale warning (hidden by default)
        this._menuStale = new PopupMenu.PopupMenuItem('', { reactive: false });
        this._menuStale.label.set_style('color: #e6961e;');
        this._menuStale.visible = false;
        this.menu.addMenuItem(this._menuStale);

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        // Claude process state
        this._menuClaudeState = new PopupMenu.PopupMenuItem('Claude: --', { reactive: false });
        this.menu.addMenuItem(this._menuClaudeState);

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        const refreshItem = new PopupMenu.PopupMenuItem('Refresh Now');
        refreshItem.connect('activate', () => this._fetchStatus(true));
        this.menu.addMenuItem(refreshItem);

        this.menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        // Disclaimer
        const disclaimer = new PopupMenu.PopupMenuItem(
            'Estimated data. Run /usage in Claude Code for exact information.',
            { reactive: false },
        );
        disclaimer.label.set_style('font-size: 10px; color: #888888;');
        this.menu.addMenuItem(disclaimer);

        // Initial fetch
        this._fetchStatus(false);

        // Periodic refresh
        this._refreshTimerId = GLib.timeout_add_seconds(GLib.PRIORITY_DEFAULT, REFRESH_SECONDS, () => {
            this._fetchStatus(false);
            return GLib.SOURCE_CONTINUE;
        });
    }

    _fetchStatus(forcePoll) {
        if (this._pending) return;

        let binaryPath = GLib.find_program_in_path(BINARY_NAME);
        if (!binaryPath) {
            const fallback = GLib.build_filenamev([GLib.get_home_dir(), '.local', 'bin', BINARY_NAME]);
            if (GLib.file_test(fallback, GLib.FileTest.IS_EXECUTABLE)) {
                binaryPath = fallback;
            }
        }
        if (!binaryPath) {
            log(`[claude-usage] '${BINARY_NAME}' not found in PATH or ~/.local/bin`);
            this._showError(`'${BINARY_NAME}' not found in PATH or ~/.local/bin`);
            return;
        }

        const args = [binaryPath, '--status'];
        if (forcePoll) args.push('--force-poll');

        try {
            this._pending = true;
            const proc = Gio.Subprocess.new(
                args,
                Gio.SubprocessFlags.STDOUT_PIPE | Gio.SubprocessFlags.STDERR_SILENCE,
            );

            proc.communicate_utf8_async(null, null, (proc, res) => {
                this._pending = false;
                try {
                    const [ok, stdout] = proc.communicate_utf8_finish(res);
                    if (!ok || !stdout || !stdout.trim()) return;
                    const data = JSON.parse(stdout.trim());
                    this._updateUI(data);
                } catch (e) {
                    log(`[claude-usage] status parse error: ${e.message}`);
                    this._showError(`Parse error: ${e.message}`);
                }
            });
        } catch (e) {
            this._pending = false;
            log(`[claude-usage] spawn error: ${e.message}`);
            this._showError(`Spawn error: ${e.message}`);
        }
    }

    _showError(message) {
        this._box.set_style(
            `border: 1px solid ${ANTHROPIC_ORANGE}; border-radius: 8px; padding: 2px 10px; margin: 3px 0;`
        );
        this._box.set_opacity(128);  // 50%
        this._cLabel.set_style(`font-size: 14px; padding-right: 4px; color: ${GREY};`);
        this._cLabel.set_text('C:--');
        this._wLabel.set_style(`font-size: 14px; color: ${GREY};`);
        this._wLabel.set_text('W:--');
        this._menuCurrent.label.set_style(`color: ${GREY};`);
        this._menuCurrent.label.set_opacity(128);
        this._menuWeekly.label.set_style(`color: ${GREY};`);
        this._menuWeekly.label.set_opacity(128);
        this._menuError.label.set_text(`Error: ${message}`);
        this._menuError.visible = true;
        this._menuClaudeState.label.set_text('Claude: unknown');
        this._menuClaudeState.label.set_style(`color: ${GREY};`);
    }

    _updateUI(data) {
        const hasError = !!data.error;
        const isStale = !!data.stale;
        const claudeRunning = !!data.claude_running;
        const cColor = hasError ? GREY : (data.c_color || GREY);
        const wColor = hasError ? GREY : (data.w_color || GREY);

        // Determine opacity: not running → 40% on whole box, stale → 50% on labels only
        if (!claudeRunning) {
            this._box.set_opacity(128);  // 50%
        } else {
            this._box.set_opacity(255);
        }
        this._box.set_style(
            `border: 1px solid ${ANTHROPIC_ORANGE}; border-radius: 8px; padding: 2px 10px; margin: 3px 0;`
        );

        const labelOpacity = (isStale && !hasError && claudeRunning) ? 128 : 255;  // 50% when stale

        this._cLabel.set_style(`font-size: 14px; padding-right: 4px; color: ${cColor};`);
        this._cLabel.set_opacity(labelOpacity);
        this._cLabel.set_text(`C:${data.c_pct}%`);

        this._wLabel.set_style(`font-size: 14px; color: ${wColor};`);
        this._wLabel.set_opacity(labelOpacity);
        this._wLabel.set_text(`W:${data.w_pct}%`);

        // Dropdown: same colors and opacity as panel labels
        this._menuCurrent.label.set_style(`color: ${cColor};`);
        this._menuCurrent.label.set_opacity(labelOpacity);
        this._menuCurrent.label.set_text(`Current (5h):  ${data.c_pct}%  resets in ${data.c_reset}`);
        this._menuWeekly.label.set_style(`color: ${wColor};`);
        this._menuWeekly.label.set_opacity(labelOpacity);
        this._menuWeekly.label.set_text(`Weekly  (7d):  ${data.w_pct}%  resets in ${data.w_reset}`);

        if (hasError) {
            this._menuError.label.set_text(`Error: ${data.error}`);
            this._menuError.visible = true;
        } else {
            this._menuError.visible = false;
        }

        if (isStale && !hasError) {
            this._menuStale.label.set_text('Cached data may be outdated');
            this._menuStale.visible = true;
        } else {
            this._menuStale.visible = false;
        }

        // Claude process state
        if (claudeRunning) {
            this._menuClaudeState.label.set_text('Claude: running');
            this._menuClaudeState.label.set_style('color: #32c850;');
        } else {
            this._menuClaudeState.label.set_text('Claude: not running');
            this._menuClaudeState.label.set_style(`color: ${ANTHROPIC_ORANGE};`);
        }
    }

    destroy() {
        if (this._refreshTimerId) {
            GLib.source_remove(this._refreshTimerId);
            this._refreshTimerId = null;
        }
        super.destroy();
    }
});

let _indicator = null;

export default class ClaudeUsageExtension extends Extension {
    enable() {
        _indicator = new ClaudeUsageIndicator(this.path);
        Main.panel.addToStatusArea('claude-usage', _indicator, 0, 'right');
    }

    disable() {
        if (_indicator) {
            _indicator.destroy();
            _indicator = null;
        }
    }
}
