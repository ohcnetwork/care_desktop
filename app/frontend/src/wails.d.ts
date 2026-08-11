// Ambient types for the Wails bridge that the runtime injects on window.
// Go methods (main.App) are exposed as window.go.main.App.<Method> returning Promises;
// events go through window.runtime.

export {};

type DockerStatus = { ok: boolean; message: string };
type NameStatus = { ok: boolean; message: string; how: string };
type NetworkStatus = { applicable: boolean; ok: boolean; message: string; how: string; fixable: boolean };
type Health = { active: boolean; code: number; detail: string };
type AppState = { setup_done: boolean; mdns_name: string; docker: DockerStatus };
type Backup = {
  db_dump: string;
  files_archive: string;
  label: string;
  manual: boolean;
  encrypted: boolean;
  size_bytes: number;
};
type CarePlugin = {
  name: string;
  package_name: string;
  version?: string;
  configs?: Record<string, unknown>;
};
type ClinicApp = {
  slug: string;
  name: string;
  description: string;
  enabled: boolean;
  managed: boolean;
  ready: boolean;
  url: string;
  warning: string;
  needs_backend_plug: string;
};
type FrontendPlugin = {
  slug: string;
  meta: Record<string, unknown>;
};

declare global {
  interface Window {
    go: {
      main: {
        App: {
          GetState(): Promise<AppState>;
          DockerStatus(): Promise<DockerStatus>;
          GitStatus(): Promise<DockerStatus>;
          MDNSStatus(): Promise<NameStatus>;
          NetworkStatus(): Promise<NetworkStatus>;
          FixNetwork(): Promise<void>;
          CareHealth(): Promise<Health>;
          ValidatePassword(pw: string): Promise<string>;
          ValidateDomain(name: string): Promise<string>;
          SetMDNSName(name: string): Promise<void>;
          VerifyAdminPassword(pw: string): Promise<boolean>;
          CareAction(action: string): Promise<void>;
          CareStatus(): Promise<string>;
          RunSetup(
            mdnsName: string,
            adminPassword: string,
            backupPassword: string,
            rememberBackup: boolean,
            installDir: string,
            backupDir: string,
          ): Promise<void>;
          CleanupFailedInstall(): Promise<void>;
          ReadEnv(name: string): Promise<string>;
          WriteEnv(name: string, content: string): Promise<void>;
          ReadPlugins(): Promise<CarePlugin[]>;
          SavePlugins(plugins: CarePlugin[]): Promise<void>;
          ListApps(): Promise<ClinicApp[]>;
          SetAppEnabled(slug: string, enabled: boolean): Promise<void>;
          ReadFrontendPlugins(): Promise<FrontendPlugin[]>;
          SaveFrontendPlugins(plugins: FrontendPlugin[]): Promise<void>;
          ListBackups(): Promise<Backup[]>;
          ConfirmRestore(filesIncluded: boolean): Promise<boolean>;
          BackupEncryptionEnabled(): Promise<boolean>;
          HasStoredBackupPassword(): Promise<boolean>;
          RestoreBackup(
            dbDump: string,
            filesArchive: string,
            passphrase: string,
            remember: boolean,
          ): Promise<void>;
          ConfirmUninstall(removeBackups: boolean): Promise<boolean>;
          RunUninstall(removeImages: boolean, removeBackups: boolean): Promise<void>;
          OpenURL(url: string): Promise<void>;
          ChooseFolder(title: string): Promise<string>;
          WasAutostartLaunched(): Promise<boolean>;
          AutostartEnabled(): Promise<boolean>;
          SetAutostart(on: boolean): Promise<void>;
        };
      };
    };
    runtime: {
      EventsOn(event: string, cb: (...data: any[]) => void): () => void;
      EventsEmit(event: string, ...data: any[]): void;
    };
  }
}
