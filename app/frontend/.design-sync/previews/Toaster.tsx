import { useEffect } from "react";
import { Toaster, toast } from "care-desktop-ui";

export const AddressCopied = () => {
  useEffect(() => {
    toast("Address copied", { duration: Infinity });
  }, []);
  return (
    <div className="relative h-[160px] w-[600px]">
      <Toaster />
    </div>
  );
};
