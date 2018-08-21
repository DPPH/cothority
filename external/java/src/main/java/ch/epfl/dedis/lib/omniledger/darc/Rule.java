package ch.epfl.dedis.lib.omniledger.darc;

import ch.epfl.dedis.proto.DarcProto;
import com.google.protobuf.ByteString;

public class Rule {
    private String action;
    private byte[] expr;

    public Rule(String action, byte[] expr) {
        this.action = action;
        this.expr = expr;
    }

    public String getAction() {
        return action;
    }

    public byte[] getExpr() {
        return expr;
    }

    public DarcProto.Rule toProto() {
        DarcProto.Rule.Builder b = DarcProto.Rule.newBuilder();
        b.setAction(this.action);
        b.setExpr(ByteString.copyFrom(this.expr));
        return b.build();
    }
}
