package knoxlink.agent;

import java.util.ArrayList;
import net.bytebuddy.asm.Advice;

public class InstallAdvice {
    @Advice.OnMethodEnter(skipOn = Advice.OnNonDefaultValue.class)
    public static boolean enter(@Advice.Argument(0) ArrayList<Long> ids) {
        return State.installFromLocal(ids);
    }

    @Advice.OnMethodExit
    public static void exit(@Advice.Enter boolean handled, @Advice.Return(readOnly = false) boolean ret) {
        if (handled) ret = true;
    }
}
