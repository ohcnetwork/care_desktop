---
category: Feedback
---

Modal confirmation dialog (Radix AlertDialog) over a blurred dark-green overlay. Compose: `<AlertDialog open onOpenChange><AlertDialogTrigger asChild>…</AlertDialogTrigger><AlertDialogContent><AlertDialogTitle>…</AlertDialogTitle><AlertDialogDescription>…</AlertDialogDescription><AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><AlertDialogAction>Confirm</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>`. `AlertDialogAction` is styled as a primary button and `AlertDialogCancel` as the default button; for a destructive confirm pass `className={buttonVariants({ variant: "destructive" })}` to the action. Widen with `AlertDialogContent className="max-w-[600px]"` (default 460px).
