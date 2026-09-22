import { LogIn } from "lucide-react";
import { useState, type FormEvent } from "react";

import { login, logout } from "@/care/auth";
import { Field } from "@/components/field";
import { Screen, ScreenBody, ScreenHead } from "@/components/screen";
import { Spinner } from "@/components/spinner";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { errorText } from "@/lib/format";
import { useWizard } from "@/state/wizard";

export function LoginScreen() {
  const { setUser, setPhase } = useWizard();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setProblem("");
    try {
      const user = await login(username.trim(), password);
      if (!user.is_superuser) {
        logout();
        setProblem("Only the CARE administrator can run the facility setup.");
        return;
      }
      setUser(user);
      setPhase("checking");
    } catch (e) {
      setProblem(errorText(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Screen>
      <ScreenHead
        title="Set up this CARE instance"
        subtitle="Sign in with the CARE administrator login created when CARE Desktop was installed."
      />
      <ScreenBody>
        <form onSubmit={submit} className="flex max-w-[420px] flex-col gap-5">
          <Field label="Username" htmlFor="username">
            <Input
              id="username"
              value={username}
              autoCapitalize="none"
              autoComplete="username"
              spellCheck={false}
              onChange={(e) => setUsername(e.target.value)}
            />
          </Field>
          <Field label="Password" htmlFor="password">
            <Input
              id="password"
              type="password"
              value={password}
              autoComplete="current-password"
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>
          {problem ? <Alert variant="danger">{problem}</Alert> : null}
          <div>
            <Button variant="primary" size="lg" type="submit" disabled={busy || !username || !password}>
              {busy ? <Spinner /> : <LogIn className="size-[17px]" strokeWidth={2.2} />}
              Sign in
            </Button>
          </div>
        </form>
      </ScreenBody>
    </Screen>
  );
}
