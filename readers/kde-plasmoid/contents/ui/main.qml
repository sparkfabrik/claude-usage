import QtQuick 2.15
import QtQuick.Layouts 1.15
import org.kde.plasma.plasmoid 2.0
import org.kde.plasma.plasma5support as P5Support
import org.kde.plasma.components 3.0 as PlasmaComponents
import org.kde.plasma.extras 2.0 as PlasmaExtras
import org.kde.kirigami as Kirigami

PlasmoidItem {
    id: root

    property var statusData: ({})
    property bool hasData: false
    property bool isStale: false
    property bool claudeRunning: true
    property string binaryPath: ""
    property string errorMessage: ""

    preferredRepresentation: compactRepresentation

    toolTipMainText: "Claude Usage"
    toolTipSubText: {
        if (!hasData) return "No data available"
        if (errorMessage) return "Error: " + errorMessage
        return "5h: " + statusData.c_pct + "% | 7d: " + statusData.w_pct + "%"
    }

    Component.onCompleted: {
        findBinary()
    }

    function findBinary() {
        whichSource.connectedSources = ["which claude-usage || echo ''"]
    }

    function pollStatus() {
        if (binaryPath === "") return
        statusSource.connectedSources = [binaryPath + " --status"]
    }

    P5Support.DataSource {
        id: whichSource
        engine: "executable"
        connectedSources: []

        onNewData: function(source, data) {
            var stdout = data["stdout"].trim()
            if (stdout && stdout !== "") {
                root.binaryPath = stdout
            } else {
                root.binaryPath = StandardPaths.home + "/.local/bin/claude-usage"
            }
            disconnectSource(source)
            pollStatus()
        }
    }

    P5Support.DataSource {
        id: statusSource
        engine: "executable"
        connectedSources: []

        onNewData: function(source, data) {
            var stdout = data["stdout"].trim()
            var exitCode = data["exit code"]
            disconnectSource(source)

            if (exitCode !== 0 || stdout === "") {
                root.hasData = false
                root.errorMessage = "CLI error (exit " + exitCode + ")"
                return
            }

            try {
                root.statusData = JSON.parse(stdout)
                root.hasData = true
                root.isStale = root.statusData.stale || false
                root.claudeRunning = root.statusData.claude_running !== false
                root.errorMessage = root.statusData.error || ""
            } catch (e) {
                root.hasData = false
                root.errorMessage = "JSON parse error"
            }
        }
    }

    Timer {
        id: pollTimer
        interval: 60000
        running: true
        repeat: true
        onTriggered: pollStatus()
    }

    compactRepresentation: Item {
        Layout.minimumWidth: label.implicitWidth
        Layout.preferredWidth: label.implicitWidth

        PlasmaComponents.Label {
            id: label
            anchors.fill: parent
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
            opacity: root.isStale ? 0.5 : (root.claudeRunning ? 1.0 : 0.3)
            visible: root.claudeRunning || root.isStale

            text: {
                if (!root.hasData) return "C:?"
                return "C:" + root.statusData.c_pct + "% W:" + root.statusData.w_pct + "%"
            }

            color: {
                if (!root.hasData) return Kirigami.Theme.textColor
                return root.statusData.c_color || Kirigami.Theme.textColor
            }
        }

        MouseArea {
            anchors.fill: parent
            onClicked: root.expanded = !root.expanded
        }
    }

    fullRepresentation: ColumnLayout {
        Layout.minimumWidth: 250
        Layout.minimumHeight: 150
        spacing: 8

        PlasmaExtras.Heading {
            level: 4
            text: "Claude Code Usage"
        }

        PlasmaComponents.Label {
            visible: root.hasData
            text: "5h utilization: " + (root.statusData.c_pct || 0) + "%"
            color: root.statusData.c_color || Kirigami.Theme.textColor
        }

        PlasmaComponents.Label {
            visible: root.hasData
            text: "Resets in: " + (root.statusData.c_reset || "?")
        }

        PlasmaComponents.Label {
            visible: root.hasData
            text: "7d utilization: " + (root.statusData.w_pct || 0) + "%"
            color: root.statusData.w_color || Kirigami.Theme.textColor
        }

        PlasmaComponents.Label {
            visible: root.hasData
            text: "Resets in: " + (root.statusData.w_reset || "?")
        }

        PlasmaComponents.Label {
            visible: root.errorMessage !== ""
            text: "Error: " + root.errorMessage
            color: "#dc3232"
        }

        PlasmaComponents.Label {
            visible: !root.hasData && root.errorMessage === ""
            text: "No data available"
        }

        PlasmaComponents.Label {
            visible: root.isStale
            text: "(data may be stale)"
            opacity: 0.6
            font.italic: true
        }
    }
}
