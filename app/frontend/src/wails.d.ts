// Ambient types for the Wails bridge that the runtime injects on window.
// Go methods (main.App) are exposed as window.go.main.App.<Method> returning
// Promises; events go through window.runtime.
import type {
  AppState,
  Backup,
  CarePlugin,
  DockerStatus,
  FrontendPlugin,
  Health,
  ImportedBackup,
  NameStatus,
  NetworkStatus,
  ResidueReport,
  RestartPlan,
  ToolPlan,
} from "./types";

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
          DockerPlan(): Promise<ToolPlan>;
          GitPlan(): Promise<ToolPlan>;
          InstallDocker(): Promise<string>;
          InstallGit(): Promise<string>;
          OpenDocker(): Promise<void>;
          ScanResidue(): Promise<ResidueReport>;
          PurgeResidue(): Promise<void>;
          RestartPlan(): Promise<RestartPlan>;
          RestartNow(): Promise<void>;
          ClinicHealth(): Promise<Health>;
          ValidatePassword(pw: string): Promise<string>;
          ValidateDomain(name: string): Promise<string>;
          ValidateBackupDir(dir: string): Promise<string>;
          SetMDNSName(name: string): Promise<void>;
          VerifyAdminPassword(pw: string): Promise<boolean>;
          ClinicAction(action: string): Promise<void>;
          ClinicStatus(): Promise<string>;
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
          ReadFrontendPlugins(): Promise<FrontendPlugin[]>;
          SaveFrontendPlugins(plugins: FrontendPlugin[]): Promise<void>;
          ListBackups(): Promise<Backup[]>;
          GetBackupDir(): Promise<string>;
          SetBackupDir(dir: string): Promise<string>;
          ChooseBackupFile(): Promise<string>;
          InspectBackupFile(path: string): Promise<ImportedBackup>;
          RestoreFromFile(
            path: string,
            passphrase: string,
            remember: boolean,
          ): Promise<void>;
          RestoreBackup(
            dbDump: string,
            filesArchive: string,
            passphrase: string,
            remember: boolean,
          ): Promise<void>;
          RunUninstall(removeImages: boolean, removeBackups: boolean): Promise<void>;
          OpenURL(url: string): Promise<void>;
          ChooseFolder(title: string): Promise<string>;
          LogPath(): Promise<string>;
          OpenLogFolder(): Promise<void>;
          WasAutostartLaunched(): Promise<boolean>;
          AutostartEnabled(): Promise<boolean>;
          SetAutostart(on: boolean): Promise<void>;
        };
      };
    };
    runtime: {
      EventsOn(event: string, cb: (...data: any[]) => void): () => void;
      EventsEmit(event: string, ...data: any[]): void;
      LogPrint(message: string): void;
    };
  }
}
