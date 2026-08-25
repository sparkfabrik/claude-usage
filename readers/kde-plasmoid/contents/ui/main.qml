import QtQuick
import QtQuick.Layouts
import org.kde.plasma.plasmoid
import org.kde.plasma.core as PlasmaCore
import org.kde.plasma.components as PlasmaComponents
import org.kde.plasma.plasma5support as Plasma5Support

PlasmoidItem {
    id: root

    property int cPct: 0
    property int wPct: 0
    property string cReset: "?"
    property string wReset: "?"
    property bool stale: false
    property bool claudeRunning: false
    property bool hasData: false
    property string errorMsg: ""
    property string authState: "unknown"

    // Colors
    readonly property string claudeOrange: "#D97757"
    readonly property string warningAmber: "#E6961E"
    readonly property string criticalRed: "#DC3232"

    Plasmoid.status: hasData && claudeRunning ? PlasmaCore.Types.ActiveStatus : PlasmaCore.Types.HiddenStatus

    function colorForPct(pct) {
        if (pct >= 95) return criticalRed;
        if (pct >= 75) return warningAmber;
        return claudeOrange;
    }

    property var modelLimits: []
    property string todayLine: ""

    // formatTokens renders a token count compactly: 282200000 becomes "282.2M".
    function formatTokens(n) {
        if (typeof n !== "number" || !isFinite(n)) return "0";
        if (n >= 1e9) return (n / 1e9).toFixed(1) + "B";
        if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
        if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
        return String(n);
    }

    function glyphForPct(pct) {
        if (pct < 50) return "◔";
        if (pct < 75) return "◑";
        if (pct < 95) return "◕";
        return "●";
    }

    function parseOutput(stdout) {
        var text = stdout.trim();
        if (!text) {
            hasData = false;
            return;
        }
        try {
            var data = JSON.parse(text);
            cPct = data.c_pct || 0;
            wPct = data.w_pct || 0;
            cReset = data.c_reset || "?";
            wReset = data.w_reset || "?";
            stale = data.stale || false;
            claudeRunning = data.claude_running !== false;
            errorMsg = data.error || "";
            authState = data.auth || "unknown";
            // Model-scoped windows arrive only from the OAuth source; the
            // header fallback and older CLI versions report none.
            modelLimits = Array.isArray(data.limits)
                ? data.limits.filter(function (limit) { return limit && limit.model; })
                : [];
            todayLine = data.today && typeof data.today.tokens === "number"
                ? "Today: " + formatTokens(data.today.tokens) + " tokens · "
                  + (data.today.messages || 0) + " messages"
                : "";
            hasData = true;
        } catch (e) {
            hasData = false;
        }
    }

    Plasma5Support.DataSource {
        id: executable
        engine: "executable"
        connectedSources: []

        onNewData: function(source, data) {
            var stdout = data["stdout"] || "";
            disconnectSource(source);
            parseOutput(stdout);
        }
    }

    function pollStatus() {
        var cmd = "claude-usage --status 2>/dev/null || $HOME/.local/bin/claude-usage --status 2>/dev/null || echo ''";
        if (executable.connectedSources.indexOf(cmd) !== -1) {
            executable.disconnectSource(cmd);
        }
        executable.connectSource(cmd);
    }

    Timer {
        id: pollTimer
        interval: 60000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: pollStatus()
    }

    preferredRepresentation: compactRepresentation

    compactRepresentation: MouseArea {
        id: compactMouse
        readonly property bool shouldShow: root.hasData && root.claudeRunning
        Layout.preferredWidth: shouldShow ? compactGrid.implicitWidth : 0
        Layout.preferredHeight: shouldShow ? compactGrid.implicitHeight : 0
        Layout.maximumWidth: shouldShow ? -1 : 0
        Layout.maximumHeight: shouldShow ? -1 : 0
        visible: shouldShow
        onClicked: root.expanded = !root.expanded

        GridLayout {
            id: compactGrid
            anchors.centerIn: parent
            columns: 2
            columnSpacing: 2
            rowSpacing: 0
            opacity: root.stale ? 0.5 : 1.0

            PlasmaComponents.Label {
                text: root.stale ? "⚠" : root.glyphForPct(root.cPct)
                color: root.claudeOrange
                font.pixelSize: 11
                Layout.rowSpan: 2
                Layout.alignment: Qt.AlignVCenter
            }

            PlasmaComponents.Label {
                text: "5h:" + root.cPct + "%"
                color: root.colorForPct(root.cPct)
                font.pixelSize: 11
            }

            PlasmaComponents.Label {
                text: "7d:" + root.wPct + "%"
                color: root.colorForPct(root.wPct)
                font.pixelSize: 11
            }
        }
    }

    fullRepresentation: ColumnLayout {
        Layout.preferredWidth: 200
        Layout.preferredHeight: implicitHeight
        spacing: 4

        PlasmaComponents.Label {
            text: "Claude Code Quota"
            font.bold: true
            font.pixelSize: 13
            color: root.claudeOrange
            Layout.bottomMargin: 2
        }

        PlasmaComponents.Label {
            text: "5h: " + root.cPct + "%  ⟳ " + root.cReset
            color: root.colorForPct(root.cPct)
            font.pixelSize: 12
        }

        PlasmaComponents.Label {
            text: "7d: " + root.wPct + "%  ⟳ " + root.wReset
            color: root.colorForPct(root.wPct)
            font.pixelSize: 12
        }

        Repeater {
            model: root.modelLimits

            PlasmaComponents.Label {
                required property var modelData
                text: modelData.title + ": " + modelData.pct + "%  ⟳ " + modelData.reset
                color: root.colorForPct(modelData.pct)
                font.pixelSize: 12
            }
        }

        PlasmaComponents.Label {
            visible: root.todayLine !== ""
            text: root.todayLine
            font.pixelSize: 11
            opacity: 0.7
        }

        PlasmaComponents.Label {
            visible: root.hasData && root.authState !== "valid"
            text: {
                if (root.authState === "expired") return "Auth expired — run Claude Code to refresh"
                if (root.authState === "missing") return "No credentials found"
                return "Auth: unknown"
            }
            color: {
                if (root.authState === "expired") return root.warningAmber
                if (root.authState === "missing") return root.criticalRed
                return "#888888"
            }
            font.pixelSize: 11
        }

        PlasmaComponents.Label {
            visible: root.errorMsg !== ""
            text: "Error: " + root.errorMsg
            font.pixelSize: 11
            opacity: 0.6
        }

        PlasmaComponents.Label {
            visible: root.stale
            text: "⚠ Data may be stale"
            color: root.warningAmber
            font.pixelSize: 11
        }
    }
}
