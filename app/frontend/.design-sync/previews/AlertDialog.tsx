import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
  buttonVariants,
} from "care-desktop-ui";

export const Confirm = () => (
  <AlertDialog open>
    <AlertDialogContent>
      <AlertDialogTitle>Restart the clinic?</AlertDialogTitle>
      <AlertDialogDescription>
        Staff will lose their connection for about a minute while CARE restarts. Unsaved forms in their
        browsers may need to be filled in again.
      </AlertDialogDescription>
      <AlertDialogFooter>
        <AlertDialogCancel>Cancel</AlertDialogCancel>
        <AlertDialogAction>Restart</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
);

export const Destructive = () => (
  <AlertDialog open>
    <AlertDialogContent>
      <AlertDialogTitle>Restore this backup?</AlertDialogTitle>
      <AlertDialogDescription>
        Everything entered since 09/09 04:30 will be replaced by the backup. This cannot be undone.
      </AlertDialogDescription>
      <AlertDialogFooter>
        <AlertDialogCancel>Cancel</AlertDialogCancel>
        <AlertDialogAction className={buttonVariants({ variant: "destructive" })}>Restore</AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
);
