package knoxlink.agent;

import static net.bytebuddy.matcher.ElementMatchers.named;

import java.lang.instrument.Instrumentation;
import net.bytebuddy.agent.builder.AgentBuilder;
import net.bytebuddy.asm.Advice;

final class Patcher {
    private Patcher() {}

    static void install(Instrumentation inst) {
        new AgentBuilder.Default()
            .disableClassFormatChanges()
            .with(AgentBuilder.RedefinitionStrategy.RETRANSFORMATION)
            .type(named("zombie.ZomboidFileSystem"))
            .transform((builder, type, loader, module, domain) ->
                builder.visit(Advice.to(FoldersAdvice.class).on(named("getInstalledItemModsFolders"))))
            .type(named("zombie.network.GameServerWorkshopItems"))
            .transform((builder, type, loader, module, domain) ->
                builder.visit(Advice.to(InstallAdvice.class).on(named("Install"))))
            .installOn(inst);
        State.log("server patches installed");
    }
}
