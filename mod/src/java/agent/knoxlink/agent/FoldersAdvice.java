package knoxlink.agent;

import java.util.List;
import net.bytebuddy.asm.Advice;

public class FoldersAdvice {
    @Advice.OnMethodExit
    public static void exit(@Advice.Argument(0) List<String> out) {
        State.addFolders(out);
    }
}
