package knoxlink.client;

import me.zed_0xff.zombie_buddy.Patch;

// CoopMaster.launchServer() adds the value of getGarbageCollector() to the hosted server's
// java command line. We swap it for a -javaagent argument that loads our server helper
// (and ZombieBuddy) into the hosted server.
@Patch(className = "zombie.network.CoopMaster", methodName = "getGarbageCollector")
public class Patch_CoopMasterGC {
    @Patch.OnExit
    public static void exit(@Patch.Return(readOnly = false) String ret) {
        ret = HostLaunch.agentArgument(ret);
    }
}
