package knoxlink.client;

import se.krka.kahlua.integration.annotations.LuaMethod;

public class LuaApi {
    // Lets the Lua side know the Java helper is running (hosted server will reuse local Workshop files).
    @LuaMethod(name = "KnoxLink_javaHelperReady", global = true)
    public static boolean javaHelperReady() {
        return HostLaunch.findAgentJar() != null && HostLaunch.findZombieBuddyJar() != null;
    }
}
