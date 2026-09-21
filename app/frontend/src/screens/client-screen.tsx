import { useState, type FormEvent } from "react";

import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { Spinner } from "@/components/spinner";
import { Alert } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { bridge } from "@/lib/bridge";
import { errorText } from "@/lib/format";
import { useCare } from "@/state/care-store";

export function ClientScreen() {
  const { clientURL, clearRole } = useCare();
  const [address, setAddress] = useState(clientURL);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [connected, setConnected] = useState(false);
  const [savedAddress, setSavedAddress] = useState(clientURL);
  const [removing, setRemoving] = useState(false);
  const [leaving, setLeaving] = useState(false);

  const goBack = async () => {
    setLeaving(true);
    setError("");
    await clearRole();
    setLeaving(false);
  };

  const removeAccess = async () => {
    setBusy(true);
    setError("");
    try {
      await bridge.DisconnectClient();
      window.location.reload();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  const connect = async (event: FormEvent) => {
    event.preventDefault();
    if (busy || !address.trim()) return;
    setBusy(true);
    setError("");
    setConnected(false);
    try {
      await bridge.ConnectClient(address.trim());
      setConnected(true);
    } catch (e) {
      setError(errorText(e));
    } finally {
      await bridge.GetState().then((state) => {
        setSavedAddress(state.client_url);
        if (state.client_url) setAddress(state.client_url);
      }).catch(() => {});
      setBusy(false);
    }
  };

  return (
    <Screen>
      <ScreenHead
        className="mx-auto w-full max-w-2xl"
        kicker="CARE Desktop · Client"
        title="Connect to your clinic"
        subtitle="Use the same network as the clinic server. Ask your clinic administrator for its address."
        onBack={savedAddress === "" ? () => void goBack() : undefined}
        backDisabled={busy || leaving}
      />
      <ScreenBody className="mx-auto w-full max-w-2xl">
        <Card className="p-6">
          <form onSubmit={(event) => void connect(event)} className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="clinic-address">Clinic address</Label>
              <Input
                id="clinic-address"
                value={address}
                onChange={(event) => {
                  setAddress(event.target.value);
                  setConnected(false);
                  setError("");
                }}
                placeholder="care.local"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
                disabled={busy || leaving}
                readOnly={savedAddress !== ""}
                required
              />
            </div>
            <p className="text-sm text-muted-foreground">
              CARE Desktop will fetch the clinic certificate. Approve your operating
              system's administrator prompt to trust it, then CARE opens in your browser.
              Only connect to a clinic address you trust.
            </p>
            {error && !removing ? <Alert variant="danger" role="alert">{error}</Alert> : null}
            {connected ? (
              <Alert role="status">
                Connected. The clinic certificate is trusted and CARE has opened in your browser.
                This address is saved for next time.
              </Alert>
            ) : null}
            <Button type="submit" variant="primary" disabled={busy || leaving || !address.trim()}>
              {busy
                ? <><Spinner /> {removing ? "Removing access" : "Connecting"} — approve any administrator prompt…</>
                : "Connect and open CARE"}
            </Button>
            <p className="text-xs text-muted-foreground">
              You can retry here at any time. To use a different clinic address,
              remove the current clinic access first.
              After removal, you can choose Server or Client again.
            </p>
            <Button type="button" disabled={busy || leaving} onClick={() => {
              setError("");
              setRemoving(true);
            }}>
              Uninstall client setup
            </Button>
          </form>
        </Card>
        <AlertDialog open={removing} onOpenChange={(open) => {
          if (!busy) setRemoving(open);
        }}>
          <AlertDialogContent>
            <AlertDialogTitle>Uninstall client setup?</AlertDialogTitle>
            <AlertDialogDescription>
              Remove this computer's client setup{savedAddress ? ` for ${savedAddress}` : ""}.
              Only the certificate CARE Desktop installed for this connection will be removed;
              other trusted certificates and all clinic data are left alone.
              Approve any administrator prompt. CARE Desktop stays installed and returns to the
              Server/Client choice. To remove the app itself, use your operating system's uninstall option.
            </AlertDialogDescription>
            {error ? <Alert className="mt-3" variant="danger" role="alert">{error}</Alert> : null}
            <AlertDialogFooter>
              <AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel>
              <AlertDialogAction
                className={buttonVariants({ variant: "destructive" })}
                disabled={busy}
                onClick={(event) => {
                  event.preventDefault();
                  void removeAccess();
                }}
              >
                {busy ? <Spinner /> : null}
                {busy ? "Removing…" : "Disconnect and remove"}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </ScreenBody>
    </Screen>
  );
}
