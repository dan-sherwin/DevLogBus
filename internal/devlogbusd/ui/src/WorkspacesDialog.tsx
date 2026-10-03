import { type FormEvent, useEffect, useState } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
  Tooltip,
} from "@mui/material";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import { useMessagesContext, useMuiPrompts } from "@dsherwin/mui-kit";
import {
  deleteWorkspace,
  readWorkspaces,
  saveWorkspace,
  workspacesStorageKey,
  type SavedWorkspace,
  type ViewerPreferences,
} from "./workspaces";

export default function WorkspacesDialog({
  onClose,
  onLoad,
  open,
  preferences,
}: {
  onClose: () => void;
  onLoad: (workspace: SavedWorkspace) => void;
  open: boolean;
  preferences: ViewerPreferences;
}) {
  const { displayErrorMessage, displaySuccessMessage } = useMessagesContext();
  const { confirmPrompt } = useMuiPrompts();
  const [workspaces, setWorkspaces] = useState<SavedWorkspace[]>(readWorkspaces);
  const [name, setName] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) {
      return;
    }
    setWorkspaces(readWorkspaces());
    setName("");
    setError("");
    const syncWorkspaces = (event: StorageEvent) => {
      if (event.key == null || event.key === workspacesStorageKey) {
        setWorkspaces(readWorkspaces());
      }
    };
    window.addEventListener("storage", syncWorkspaces);
    return () => window.removeEventListener("storage", syncWorkspaces);
  }, [open]);

  const save = (event: FormEvent) => {
    event.preventDefault();
    try {
      setWorkspaces(saveWorkspace(name, preferences));
      displaySuccessMessage(`Saved ${name.trim()}`);
      setName("");
      setError("");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Unable to save workspace.");
    }
  };

  const load = (name: string) => {
    const workspace = readWorkspaces().find((saved) => saved.name === name);
    if (workspace == null) {
      setWorkspaces(readWorkspaces());
      displayErrorMessage("That workspace no longer exists.");
      return;
    }
    onLoad(workspace);
    onClose();
    displaySuccessMessage(`Loaded ${workspace.name}`);
  };

  const update = async (workspace: SavedWorkspace) => {
    if (!(await confirmPrompt({
      title: "Update workspace?",
      message: `Replace the saved settings for “${workspace.name}” with the current viewer settings?`,
      buttonText: "Update",
    }))) {
      return;
    }
    try {
      setWorkspaces(saveWorkspace(workspace.name, preferences, true));
      displaySuccessMessage(`Updated ${workspace.name}`);
    } catch (cause) {
      displayErrorMessage(cause instanceof Error ? cause.message : "Unable to update workspace.");
    }
  };

  const remove = async (workspace: SavedWorkspace) => {
    if (!(await confirmPrompt({
      title: "Delete workspace?",
      message: `Delete “${workspace.name}”? Your current viewer settings will stay as they are.`,
      buttonText: "Delete",
      buttonColor: "error",
    }))) {
      return;
    }
    try {
      setWorkspaces(deleteWorkspace(workspace.name));
      displaySuccessMessage(`Deleted ${workspace.name}`);
    } catch (cause) {
      displayErrorMessage(cause instanceof Error ? cause.message : "Unable to delete workspace.");
    }
  };

  return (
    <Dialog className="appDialog" fullWidth maxWidth="sm" onClose={onClose} open={open}>
      <DialogTitle>Investigation Workspaces</DialogTitle>
      <DialogContent>
        <p className="workspaceIntro">
          Save your layout, source visibility, blocked sources, filters, search, and pane
          preferences for the next investigation. Workspaces stay in this browser.
        </p>
        <form className="workspaceSave" onSubmit={save}>
          <TextField
            error={error !== ""}
            fullWidth
            helperText={error}
            label="Workspace name"
            onChange={(event) => {
              setName(event.target.value);
              setError("");
            }}
            size="small"
            value={name}
          />
          <Button disabled={name.trim() === ""} type="submit" variant="contained">
            Save current
          </Button>
        </form>
        {workspaces.length === 0 ? (
          <div className="emptyState">No saved workspaces yet.</div>
        ) : (
          <div className="workspaceList">
            {workspaces.map((workspace) => (
              <div className="workspaceRow" key={workspace.name}>
                <div className="workspaceDescription">
                  <strong>{workspace.name}</strong>
                  <span>
                    {workspace.preferences.viewMode === "merged" ? "Merged" : "By source"}
                    {workspace.preferences.viewMode === "source" && ` · ${workspace.preferences.sourceLayout}`}
                    {` · ${workspace.preferences.excludedSources.length} hidden · ${workspace.preferences.blockedSources.length} blocked`}
                  </span>
                </div>
                <div className="workspaceActions">
                  <Button onClick={() => load(workspace.name)} size="small" variant="outlined">
                    Load
                  </Button>
                  <Button onClick={() => void update(workspace)} size="small">
                    Update
                  </Button>
                  <Tooltip title={`Delete ${workspace.name}`}>
                    <IconButton
                      aria-label={`Delete ${workspace.name}`}
                      onClick={() => void remove(workspace)}
                      size="small"
                    >
                      <DeleteOutlinedIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                </div>
              </div>
            ))}
          </div>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} variant="contained">
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
}
